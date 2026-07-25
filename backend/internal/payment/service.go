package payment

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"pagasacentre/backend/internal/email"
	commonerrors "pagasacentre/backend/pkg/commonlibrary/errors"
	"pagasacentre/backend/internal/registration/domain"
	"pagasacentre/backend/internal/registration/storage"
	"pagasacentre/backend/internal/sheets"
)

// Service handles Stripe webhook events. As of v2 it is a thin layer that
// transitions a registration_group to 'paid' and fires the confirmation email.
// Race-loser refund logic was deleted along with accommodation capacity caps.
type Service struct {
	pool          *pgxpool.Pool
	regRepo       *storage.Repository
	mailer        email.Mailer
	sheets        sheets.Sync
	publicBaseURL string
}

func NewService(pool *pgxpool.Pool, regRepo *storage.Repository, mailer email.Mailer, sheetSync sheets.Sync, publicBaseURL string) *Service {
	if sheetSync == nil {
		sheetSync = sheets.NewNoopSync()
	}
	return &Service{
		pool:          pool,
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
	if group.PaymentStatus != domain.PaymentPending {
		return nil // already processed — webhook replays are expected
	}

	if group.PaidInFullAtRegistration {
		coachIncluded, err := s.groupHasCoachEligible(ctx, group.ID)
		if err != nil {
			return err
		}
		if err := s.regRepo.MarkPaidInFull(ctx, tx, group.ID, evt.PaymentIntentID, coachIncluded); err != nil {
			return err
		}
	} else if err := s.regRepo.MarkPaid(ctx, tx, group.ID, evt.PaymentIntentID); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	committed = true

	// MarkPaid set payment_status='paid' and paid_at=now() in the DB. Mirror
	// that into the in-memory group so sendConfirmationEmail + syncToSheet
	// see consistent data without an extra round-trip.
	now := time.Now().UTC()
	group.PaymentStatus = domain.PaymentPaid
	group.PaidAt = &now

	s.sendConfirmationEmail(ctx, group)
	s.syncToSheet(ctx, group)
	return nil
}

// syncToSheet appends the paid group + campers to the Paid tab and deletes
// any matching Pending rows. Failure is logged, never fatal — the webhook
// return value of "OK" already triggered DB commit + email; the sheet is a
// view layer.
func (s *Service) syncToSheet(ctx context.Context, g *domain.Group) {
	if s.sheets == nil {
		return
	}
	campers, err := s.regRepo.CampersForGroup(ctx, g.ID)
	if err != nil {
		log.Printf("sheets: load campers for group %s: %v", g.ID, err)
		return
	}
	rows := rowsFromGroup(g, campers)
	if err := s.sheets.AppendPaidAndRemovePending(ctx, g.ID, rows); err != nil {
		log.Printf("sheets: append paid for group %s: %v", g.ID, err)
	}
}

// rowsFromGroup builds one sheets.Row per camper, denormalising group-level
// contact fields onto each row to match the CSV export layout.
func rowsFromGroup(g *domain.Group, campers []domain.Camper) []sheets.Row {
	rows := make([]sheets.Row, 0, len(campers))
	for _, c := range campers {
		rows = append(rows, sheets.Row{
			GroupID:                   g.ID,
			PaymentStatus:             g.PaymentStatus,
			SubmittedAt:               g.CreatedAt,
			PaidAt:                    g.PaidAt,
			TotalAmountPence:          g.TotalAmountPence,
			Currency:                  g.Currency,
			ContactFirstName:          g.ContactFirstName,
			ContactLastName:           g.ContactLastName,
			ContactEmail:              g.ContactEmail,
			ContactPhone:              g.ContactPhone,
			IsMainContact:             c.IsMainContact,
			FirstName:                 c.FirstName,
			LastName:                  c.LastName,
			Gender:                    c.Gender,
			Age:                       c.Age,
			CellLeaderName:            c.CellLeaderName,
			IsCellLeader:              c.IsCellLeader,
			AttendanceType:            c.AttendanceType,
			ShirtSize:                 c.ShirtSize,
			DietaryRequirements:       c.DietaryRequirements,
			NeedsCoach:                c.NeedsCoach,
			AccommodationFirstChoice:  c.AccommodationFirstChoice,
			AccommodationSecondChoice: c.AccommodationSecondChoice,
			RoommateRequests:          c.RoommateRequests,
			DayPassDays:               c.DayPassDays,
			DayPassTshirtOption:       c.DayPassTshirtOption,
			DayPassNeedsCatering:      c.DayPassNeedsCatering,
		})
	}
	return rows
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
	if group == nil || group.PaymentStatus != domain.PaymentPending {
		return nil
	}
	if err := s.regRepo.MarkStatusInTx(ctx, tx, group.ID, domain.PaymentCancelled); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// sendConfirmationEmail dispatches the deposit confirmation. Failure is
// logged, never surfaced — the registration is already paid in DB at this
// point, and a webhook return value of error would cause Stripe to retry the
// whole HandleCheckoutCompleted (which is fine, but doesn't help if SMTP is
// down — we'd just rack up retries).
func (s *Service) sendConfirmationEmail(ctx context.Context, g *domain.Group) {
	if err := s.sendDepositConfirmation(ctx, g); err != nil {
		log.Printf("send confirmation email to %s failed: %v", g.ContactEmail, err)
	}
}

// sendDepositConfirmation builds and sends the deposit-received email from the
// group's current data. Returns any error so callers that need to surface
// failure (e.g. an admin re-send) can, while the webhook path keeps logging
// and swallowing via sendConfirmationEmail.
func (s *Service) sendDepositConfirmation(ctx context.Context, g *domain.Group) error {
	if s.mailer == nil {
		return nil
	}
	campers, err := s.regRepo.CampersForGroup(ctx, g.ID)
	if err != nil {
		return fmt.Errorf("load campers for email (group %s): %w", g.ID, err)
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
	return s.mailer.SendDepositConfirmation(ctx, email.DepositConfirmation{
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
	for _, c := range campers {
		if c.AttendanceType != domain.AttendanceFullWeek {
			continue
		}
		if c.NeedsCoach != nil && *c.NeedsCoach && c.Age >= 4 {
			return true, nil
		}
	}
	return false, nil
}

// UpdateGroupContact corrects a group's contact details (e.g. a mistyped email
// captured at registration). It updates the database, mirrors the change into
// the Google Sheet, and optionally re-sends the deposit confirmation email to
// the corrected address.
//
// The DB update is the only hard requirement. The sheet update is best-effort
// (logged, not fatal — the DB is the source of truth). When resend is true the
// email failure IS surfaced, so the White Team knows whether the family was
// actually re-notified.
func (s *Service) UpdateGroupContact(ctx context.Context, groupID, firstName, lastName, emailAddr, phone string, resend bool) error {
	g, err := s.regRepo.FindGroupByID(ctx, groupID)
	if err != nil {
		return fmt.Errorf("load group: %w", err)
	}
	if g == nil {
		return commonerrors.NotFound("registration not found")
	}
	if resend && g.PaymentStatus != domain.PaymentPaid {
		return commonerrors.BadRequest(
			"the confirmation email can only be re-sent once the deposit is paid", nil)
	}

	if err := s.regRepo.UpdateContact(ctx, groupID, firstName, lastName, emailAddr, phone); err != nil {
		return fmt.Errorf("update contact: %w", err)
	}

	if s.sheets != nil {
		if err := s.sheets.UpdateContactByGroupID(ctx, groupID, sheets.ContactUpdate{
			FirstName: firstName,
			LastName:  lastName,
			Email:     emailAddr,
			Phone:     phone,
		}); err != nil {
			log.Printf("sheets: update contact for group %s failed: %v", groupID, err)
		}
	}

	if resend {
		// Reflect the corrected details in-memory so the email is addressed
		// and personalised correctly.
		g.ContactFirstName = firstName
		g.ContactLastName = lastName
		g.ContactEmail = emailAddr
		g.ContactPhone = phone
		if err := s.sendDepositConfirmation(ctx, g); err != nil {
			return fmt.Errorf(
				"details were updated, but the confirmation email failed to send: %w", err)
		}
	}
	return nil
}

// ErrUnhandledEvent is returned by the handler for event types it ignores.
var ErrUnhandledEvent = errors.New("unhandled event type")
