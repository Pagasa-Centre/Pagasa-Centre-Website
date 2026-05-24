package registration

import (
	"context"
	"fmt"
	"log"

	"github.com/jackc/pgx/v5"

	"pagasacentre/backend/internal/email"
	"pagasacentre/backend/internal/httpx"
)

// PriceLookup returns the current amount and currency for a price code.
type PriceLookup interface {
	GetPrice(ctx context.Context, code string) (PriceRow, error)
}

// PriceRow is what PriceLookup returns. We use a local type so registration
// doesn't import the camp package directly.
type PriceRow struct {
	AmountPence int
	Currency    string
}

// CheckoutCreator is implemented by the Stripe client. It's defined in this
// package so the service depends only on the small surface it actually uses.
type CheckoutCreator interface {
	CreateCheckoutSession(ctx context.Context, p CheckoutParams) (CheckoutSession, error)
}

type CheckoutParams struct {
	GroupID     string
	Email       string
	AmountPence int64
	Currency    string
	Description string
}

type CheckoutSession struct {
	ID  string
	URL string
}

// CampConfigReader returns the runtime camp config (used to check that
// registrations are open).
type CampConfigReader interface {
	RegistrationsOpen(ctx context.Context) (bool, error)
}

type Service struct {
	repo          *Repository
	prices        PriceLookup
	stripe        CheckoutCreator
	camp          CampConfigReader
	mailer        email.Mailer
	publicBaseURL string
}

func NewService(
	repo *Repository,
	prices PriceLookup,
	stripe CheckoutCreator,
	campCfg CampConfigReader,
	mailer email.Mailer,
	publicBaseURL string,
) *Service {
	return &Service{
		repo:          repo,
		prices:        prices,
		stripe:        stripe,
		camp:          campCfg,
		mailer:        mailer,
		publicBaseURL: publicBaseURL,
	}
}

// Submit validates the request, persists the pending group and campers, then
// either:
//   - creates a Stripe Checkout session (total > 0), or
//   - marks the group paid + sends a confirmation email inline (total == 0,
//     i.e. day-pass-only registrations).
func (s *Service) Submit(ctx context.Context, req SubmitRequest) (*SubmitResponse, error) {
	if err := Validate(req); err != nil {
		return nil, err
	}

	open, err := s.camp.RegistrationsOpen(ctx)
	if err != nil {
		return nil, httpx.Internal(err.Error())
	}
	if !open {
		return nil, httpx.APIError{
			Code:    "registrations_closed",
			Message: "Camp registrations are currently closed",
		}
	}

	total, currency, err := s.computeTotal(ctx, req)
	if err != nil {
		return nil, httpx.Internal(err.Error())
	}

	tx, err := s.repo.Pool().BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, httpx.Internal(err.Error())
	}
	defer func() { _ = tx.Rollback(ctx) }()

	groupID, err := s.repo.InsertGroup(ctx, tx, req, total, currency)
	if err != nil {
		return nil, httpx.Internal(err.Error())
	}
	for _, c := range req.Campers {
		if err := s.repo.InsertCamper(ctx, tx, groupID, c); err != nil {
			return nil, httpx.Internal(err.Error())
		}
	}

	hasMinor := HasMinor(req)
	resp := &SubmitResponse{
		GroupID:          groupID,
		TotalAmountPence: total,
		HasMinor:         hasMinor,
	}
	if hasMinor {
		resp.ConsentFormURL = s.publicBaseURL + "/api/consent-form"
	}

	if total == 0 {
		// Day-pass-only registration: no Stripe round-trip needed. Mark paid
		// immediately and fire the confirmation email so the user gets the
		// same "what to expect next" copy as deposit-paying groups.
		if err := s.repo.MarkPaid(ctx, tx, groupID, ""); err != nil {
			return nil, httpx.Internal(err.Error())
		}
		if err := tx.Commit(ctx); err != nil {
			return nil, httpx.Internal(err.Error())
		}
		s.sendConfirmationEmail(ctx, req, total, currency, hasMinor)
		return resp, nil
	}

	paying := depositPayingCount(req)
	session, err := s.stripe.CreateCheckoutSession(ctx, CheckoutParams{
		GroupID:     groupID,
		Email:       req.Contact.Email,
		AmountPence: int64(total),
		Currency:    currency,
		Description: fmt.Sprintf("PC Summer Camp 2026 non-refundable deposit (%d camper%s)", paying, pluralS(paying)),
	})
	if err != nil {
		return nil, httpx.Internal(fmt.Sprintf("stripe checkout: %s", err.Error()))
	}
	if err := s.repo.SetStripeSession(ctx, tx, groupID, session.ID); err != nil {
		return nil, httpx.Internal(err.Error())
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, httpx.Internal(err.Error())
	}

	resp.CheckoutURL = session.URL
	return resp, nil
}

// computeTotal returns the £-deposit total for the group: flat per
// deposit-paying camper (full-week AND age >= MinDepositAge). Day-pass
// campers and under-3s contribute 0.
func (s *Service) computeTotal(ctx context.Context, req SubmitRequest) (totalPence int, currency string, err error) {
	deposit, err := s.prices.GetPrice(ctx, PriceDeposit)
	if err != nil {
		return 0, "", fmt.Errorf("lookup deposit price: %w", err)
	}
	currency = deposit.Currency
	if currency == "" {
		currency = "GBP"
	}
	totalPence = deposit.AmountPence * depositPayingCount(req)
	return totalPence, currency, nil
}

// sendConfirmationEmail dispatches the deposit / day-pass confirmation. Failure
// is logged but not surfaced to the caller — the registration is already
// committed at this point, and we'd rather a user see "success" + chase a
// missing email than have the submit appear to fail after the fact.
func (s *Service) sendConfirmationEmail(ctx context.Context, req SubmitRequest, totalPence int, currency string, hasMinor bool) {
	if s.mailer == nil {
		return
	}
	consentURL := ""
	if hasMinor {
		consentURL = s.publicBaseURL + "/api/consent-form"
	}
	err := s.mailer.SendDepositConfirmation(ctx, email.DepositConfirmation{
		ToEmail:        req.Contact.Email,
		ToName:         req.Contact.FirstName,
		AmountPence:    totalPence,
		Currency:       currency,
		CamperCount:    len(req.Campers),
		HasMinor:       hasMinor,
		ConsentFormURL: consentURL,
	})
	if err != nil {
		log.Printf("send confirmation email to %s failed: %v", req.Contact.Email, err)
	}
}

// MinDepositAge is the youngest age that has to pay the deposit. Campers
// under this age attend free (cot / lap-of-parent etc.).
const MinDepositAge = 3

// depositPayingCount returns the number of campers in the request who owe a
// deposit: full-week AND age >= MinDepositAge.
func depositPayingCount(req SubmitRequest) int {
	n := 0
	for _, c := range req.Campers {
		if c.Attendance.Type == AttendanceFullWeek && c.Age >= MinDepositAge {
			n++
		}
	}
	return n
}

func pluralS(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
