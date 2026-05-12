package registration

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"

	"pagasacentre/backend/internal/accommodation"
	"pagasacentre/backend/internal/httpx"
)

// AccommodationLister is the subset of accommodation.Service the registration
// service needs. Defined here so we can mock it in tests.
type AccommodationLister interface {
	ListAvailability(ctx context.Context) ([]accommodation.Availability, error)
}

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
	repo            *Repository
	accommodations  AccommodationLister
	prices          PriceLookup
	stripe          CheckoutCreator
	camp            CampConfigReader
	publicBaseURL   string
}

func NewService(
	repo *Repository,
	accommodations AccommodationLister,
	prices PriceLookup,
	stripe CheckoutCreator,
	campCfg CampConfigReader,
	publicBaseURL string,
) *Service {
	return &Service{
		repo:           repo,
		accommodations: accommodations,
		prices:         prices,
		stripe:         stripe,
		camp:           campCfg,
		publicBaseURL:  publicBaseURL,
	}
}

// Submit validates the request, pre-checks capacity, inserts the pending group
// and campers, creates a Stripe Checkout session, and returns the URL the
// frontend should redirect to.
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

	avail, err := s.accommodations.ListAvailability(ctx)
	if err != nil {
		return nil, httpx.Internal(err.Error())
	}
	if soldOut := collectSoldOut(req, avail); len(soldOut) > 0 {
		return nil, httpx.Conflict("accommodation_sold_out",
			"One or more accommodations are sold out",
			map[string]string{"accommodation_codes": strings.Join(soldOut, ",")})
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

	session, err := s.stripe.CreateCheckoutSession(ctx, CheckoutParams{
		GroupID:     groupID,
		Email:       req.Contact.Email,
		AmountPence: int64(total),
		Currency:    currency,
		Description: "PC Summer Camp 2026 Registration",
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

	hasMinor := HasMinor(req)
	resp := &SubmitResponse{
		GroupID:     groupID,
		CheckoutURL: session.URL,
		HasMinor:    hasMinor,
	}
	if hasMinor {
		resp.ConsentFormURL = s.publicBaseURL + "/api/consent-form"
	}
	return resp, nil
}

// computeTotal sums all priced line items implied by the request.
func (s *Service) computeTotal(ctx context.Context, req SubmitRequest) (totalPence int, currency string, err error) {
	prices := map[string]PriceRow{}
	get := func(code string) (PriceRow, error) {
		if p, ok := prices[code]; ok {
			return p, nil
		}
		p, err := s.prices.GetPrice(ctx, code)
		if err != nil {
			return PriceRow{}, err
		}
		prices[code] = p
		return p, nil
	}

	currency = "GBP"
	for _, c := range req.Campers {
		switch c.Attendance.Type {
		case AttendanceFullWeek:
			code := PriceFullWeekAdult
			if c.Age < 18 {
				code = PriceFullWeekChild
			}
			p, err := get(code)
			if err != nil {
				return 0, "", err
			}
			currency = p.Currency
			totalPence += p.AmountPence
		case AttendanceDayPass:
			p, err := get(PriceDayPass)
			if err != nil {
				return 0, "", err
			}
			currency = p.Currency
			totalPence += p.AmountPence * len(c.Attendance.Days)
			if c.Attendance.TshirtOption == TshirtOptionTeamActivities ||
				c.Attendance.TshirtOption == TshirtOptionTshirtOnly {
				tp, err := get(PriceTshirtOnly)
				if err != nil {
					return 0, "", err
				}
				totalPence += tp.AmountPence
			}
		}
	}
	return totalPence, currency, nil
}

// collectSoldOut compares requested accommodations against current availability
// and returns the codes that have no room. Returned list is sorted and unique.
func collectSoldOut(req SubmitRequest, avail []accommodation.Availability) []string {
	byCode := map[string]accommodation.Availability{}
	for _, a := range avail {
		byCode[a.Code] = a
	}
	want := map[string]int{}
	for _, c := range req.Campers {
		if c.Attendance.Type == AttendanceFullWeek && c.Attendance.AccommodationCode != "" {
			want[c.Attendance.AccommodationCode]++
		}
	}
	var soldOut []string
	for code, n := range want {
		a, ok := byCode[code]
		if !ok {
			soldOut = append(soldOut, code)
			continue
		}
		if !a.HasRoomFor(n) {
			soldOut = append(soldOut, code)
		}
	}
	sort.Strings(soldOut)
	return soldOut
}
