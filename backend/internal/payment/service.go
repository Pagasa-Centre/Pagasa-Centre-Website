package payment

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"pagasacentre/backend/internal/notify"
	"pagasacentre/backend/internal/registration/domain"
)

type depositMailer interface {
	SendDepositConfirmation(ctx context.Context, p notify.DepositConfirmation) error
}

// Service handles Stripe webhook events. As of v2 it is a thin layer that
// transitions a registration_group to 'paid' and fires the confirmation email.
// Race-loser refund logic was deleted along with accommodation capacity caps.
type Service struct {
	tx            txRunner
	regRepo       regRepo
	mailer        depositMailer
	sheets        notify.SheetSync
	publicBaseURL string
}

func NewService(tx txRunner, regRepo regRepo, mailer depositMailer, sheetSync notify.SheetSync, publicBaseURL string) *Service {
	if sheetSync == nil {
		sheetSync = notify.NewNoopSheetSync()
	}
	return &Service{
		tx:            tx,
		regRepo:       regRepo,
		mailer:        mailer,
		sheets:        sheetSync,
		publicBaseURL: publicBaseURL,
	}
}

// CheckoutCompleted is the payload extracted from a checkout.session.completed
// event.
type CheckoutCompleted struct {
	SessionID       string
	PaymentIntentID string
}

func (s *Service) HandleCheckoutCompleted(ctx context.Context, evt CheckoutCompleted) error {
	var group *domain.Group
	processed := false
	err := s.tx.WithTx(ctx, func(txCtx context.Context) error {
		g, err := s.regRepo.GetGroupBySession(txCtx, evt.SessionID)
		if err != nil {
			return err
		}
		if g == nil {
			return fmt.Errorf("no registration group for session %s", evt.SessionID)
		}
		settlement, ok := g.SettleCheckout()
		if !ok {
			return nil
		}
		coachIncluded := false
		if g.PaidInFullAtRegistration {
			var err error
			coachIncluded, err = s.groupHasCoachEligible(txCtx, g.ID)
			if err != nil {
				return err
			}
		}
		if err := s.regRepo.ApplySettlement(txCtx, g.ID, evt.PaymentIntentID, coachIncluded, settlement); err != nil {
			return err
		}
		group = g
		processed = true
		return nil
	})
	if err != nil || !processed || group == nil {
		return err
	}
	now := time.Now().UTC()
	group.MarkDepositPaid(now)
	s.sendConfirmationEmail(ctx, group)
	s.syncToSheet(ctx, group)
	return nil
}

func (s *Service) syncToSheet(ctx context.Context, g *domain.Group) {
	if s.sheets == nil {
		return
	}
	campers, err := s.regRepo.CampersForGroup(ctx, g.ID)
	if err != nil {
		log.Printf("sheets: load campers for group %s: %v", g.ID, err)
		return
	}
	rows := notify.RowsForGroup(g, campers)
	if err := s.sheets.AppendPaidAndRemovePending(ctx, g.ID, rows); err != nil {
		log.Printf("sheets: append paid for group %s: %v", g.ID, err)
	}
}

func (s *Service) HandleCheckoutExpired(ctx context.Context, sessionID string) error {
	return s.tx.WithTx(ctx, func(txCtx context.Context) error {
		group, err := s.regRepo.GetGroupBySession(txCtx, sessionID)
		if err != nil {
			return err
		}
		if group == nil {
			return nil
		}
		tr, ok := group.ExpireCheckout()
		if !ok {
			return nil
		}
		return s.regRepo.SetPaymentStatus(txCtx, group.ID, tr)
	})
}

func (s *Service) sendConfirmationEmail(ctx context.Context, g *domain.Group) {
	if err := s.sendDepositConfirmation(ctx, g); err != nil {
		log.Printf("send confirmation email to %s failed: %v", g.ContactEmail, err)
	}
}

func (s *Service) sendDepositConfirmation(ctx context.Context, g *domain.Group) error {
	if s.mailer == nil {
		return nil
	}
	campers, err := s.regRepo.CampersForGroup(ctx, g.ID)
	if err != nil {
		return fmt.Errorf("load campers for email (group %s): %w", g.ID, err)
	}
	hasMinor := domain.HasMinor(campers)
	consentURL := ""
	if hasMinor {
		consentURL = s.publicBaseURL + "/api/consent-form"
	}
	return s.mailer.SendDepositConfirmation(ctx, notify.DepositConfirmation{
		ToEmail:        g.ContactEmail,
		ToName:         g.ContactFirstName,
		AmountPence:    g.TotalAmountPence,
		Currency:       g.Currency,
		CamperCount:    len(campers),
		HasMinor:       hasMinor,
		ConsentFormURL: consentURL,
		PaidInFull:     g.PaidInFullAtRegistration,
	})
}

func (s *Service) groupHasCoachEligible(ctx context.Context, groupID string) (bool, error) {
	campers, err := s.regRepo.CampersForGroup(ctx, groupID)
	if err != nil {
		return false, err
	}
	return domain.CoachEligibleCount(campers) > 0, nil
}

func (s *Service) UpdateGroupContact(ctx context.Context, groupID, firstName, lastName, emailAddr, phone string, resend bool) error {
	g, err := s.regRepo.FindGroupByID(ctx, groupID)
	if err != nil {
		return fmt.Errorf("load group: %w", err)
	}
	if g == nil {
		return domain.ErrRegistrationNotFound
	}
	if resend {
		if err := g.CanResendDepositConfirmation(); err != nil {
			return err
		}
	}

	if err := s.regRepo.UpdateContact(ctx, groupID, firstName, lastName, emailAddr, phone); err != nil {
		return fmt.Errorf("update contact: %w", err)
	}

	if s.sheets != nil {
		if err := s.sheets.UpdateContactByGroupID(ctx, groupID, notify.ContactUpdate{
			FirstName: firstName,
			LastName:  lastName,
			Email:     emailAddr,
			Phone:     phone,
		}); err != nil {
			log.Printf("sheets: update contact for group %s failed: %v", groupID, err)
		}
	}

	if resend {
		g.ChangeContact(firstName, lastName, emailAddr, phone)
		if err := s.sendDepositConfirmation(ctx, g); err != nil {
			return fmt.Errorf(
				"details were updated, but the confirmation email failed to send: %w", err)
		}
	}
	return nil
}

// ErrUnhandledEvent is returned by the handler for event types it ignores.
var ErrUnhandledEvent = errors.New("unhandled event type")
