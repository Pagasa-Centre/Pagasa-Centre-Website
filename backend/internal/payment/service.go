package payment

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"pagasacentre/backend/internal/email"
	"pagasacentre/backend/internal/registration"
)

// Service handles Stripe webhook events. As of v2 it is a thin layer that
// transitions a registration_group to 'paid' and fires the confirmation email.
// Race-loser refund logic was deleted along with accommodation capacity caps.
type Service struct {
	pool          *pgxpool.Pool
	regRepo       *registration.Repository
	mailer        email.Mailer
	publicBaseURL string
}

func NewService(pool *pgxpool.Pool, regRepo *registration.Repository, mailer email.Mailer, publicBaseURL string) *Service {
	return &Service{
		pool:          pool,
		regRepo:       regRepo,
		mailer:        mailer,
		publicBaseURL: publicBaseURL,
	}
}

// CheckoutCompleted is the payload extracted from a checkout.session.completed
// event.
type CheckoutCompleted struct {
	SessionID       string
	PaymentIntentID string
}

// HandleCheckoutCompleted transitions a pending group to paid, then sends the
// deposit confirmation email. Idempotent: replays after the first successful
// run are no-ops (email is not re-sent on the same group).
func (s *Service) HandleCheckoutCompleted(ctx context.Context, evt CheckoutCompleted) error {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback(ctx)
		}
	}()

	group, err := s.regRepo.GetGroupBySession(ctx, tx, evt.SessionID)
	if err != nil {
		return err
	}
	if group == nil {
		return fmt.Errorf("no registration group for session %s", evt.SessionID)
	}
	if group.PaymentStatus != registration.PaymentPending {
		return nil // already processed — webhook replays are expected
	}

	if err := s.regRepo.MarkPaid(ctx, tx, group.ID, evt.PaymentIntentID); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	committed = true

	s.sendConfirmationEmail(ctx, group)
	return nil
}

// HandleCheckoutExpired marks a group as cancelled when the Stripe session
// expires without payment.
func (s *Service) HandleCheckoutExpired(ctx context.Context, sessionID string) error {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	group, err := s.regRepo.GetGroupBySession(ctx, tx, sessionID)
	if err != nil {
		return err
	}
	if group == nil || group.PaymentStatus != registration.PaymentPending {
		return nil
	}
	if err := s.regRepo.MarkStatusInTx(ctx, tx, group.ID, registration.PaymentCancelled); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// sendConfirmationEmail dispatches the deposit confirmation. Failure is
// logged, never surfaced — the registration is already paid in DB at this
// point, and a webhook return value of error would cause Stripe to retry the
// whole HandleCheckoutCompleted (which is fine, but doesn't help if SMTP is
// down — we'd just rack up retries).
func (s *Service) sendConfirmationEmail(ctx context.Context, g *registration.Group) {
	if s.mailer == nil {
		return
	}
	campers, err := s.regRepo.CampersForGroup(ctx, g.ID)
	if err != nil {
		log.Printf("load campers for email (group %s): %v", g.ID, err)
		return
	}
	hasMinor := false
	for _, c := range campers {
		if c.Age < 18 {
			hasMinor = true
			break
		}
	}
	consentURL := ""
	if hasMinor {
		consentURL = s.publicBaseURL + "/api/consent-form"
	}
	if err := s.mailer.SendDepositConfirmation(ctx, email.DepositConfirmation{
		ToEmail:        g.ContactEmail,
		ToName:         g.ContactFirstName,
		AmountPence:    g.TotalAmountPence,
		Currency:       g.Currency,
		CamperCount:    len(campers),
		HasMinor:       hasMinor,
		ConsentFormURL: consentURL,
	}); err != nil {
		log.Printf("send confirmation email to %s failed: %v", g.ContactEmail, err)
	}
}

// ErrUnhandledEvent is returned by the handler for event types it ignores.
var ErrUnhandledEvent = errors.New("unhandled event type")
