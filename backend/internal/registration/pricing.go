package registration

import (
	"context"
	"fmt"
	"strings"

	"pagasacentre/backend/internal/registration/domain"
	commonerrors "pagasacentre/backend/pkg/commonlibrary/errors"
)

type StripePriceAmounts interface {
	Amount(ctx context.Context, priceID string) (int64, string, error)
}

type Config struct {
	StripePriceChildUnder3 string
	StripePriceDayPass     string
	StripePriceCoach       string
}

// CheckoutLine is one dynamic Stripe Checkout line item.
type CheckoutLine struct {
	Description string
	AmountPence int64 // unit amount
	Quantity    int64
}

type charge struct {
	totalPence int
	currency   string
	lines      []CheckoutLine
}

// PricingTier is one accommodation tier's public price.
type PricingTier struct {
	Code        string `json:"code"`
	DisplayName string `json:"display_name"`
	AmountPence int    `json:"amount_pence"`
}

// PricingSnapshot is the public price table for the registration form.
type PricingSnapshot struct {
	Mode                   string        `json:"mode"`
	Currency               string        `json:"currency"`
	DepositAmountPence     int           `json:"deposit_amount_pence"`
	AccommodationTiers     []PricingTier `json:"accommodation_tiers"`
	ChildUnder3AmountPence int           `json:"child_under3_amount_pence"`
	DayPassAmountPence     int           `json:"day_pass_amount_pence"`
	CoachAmountPence       int           `json:"coach_amount_pence"`
}

// AccommodationPriceID mirrors the billing resolver so both modes price a
// given tier + age from the same Stripe Price.
func AccommodationPriceID(code string, age int, tier *domain.AccommodationType, childUnder3PriceID string) (string, error) {
	if code == AccommodationChild && age < MinDepositAge {
		if childUnder3PriceID == "" {
			return "", fmt.Errorf("STRIPE_PRICE_CHILD_UNDER3 is not configured")
		}
		return childUnder3PriceID, nil
	}
	if tier == nil {
		return "", fmt.Errorf("unknown accommodation %q", code)
	}
	if tier.StripePriceID == nil || strings.TrimSpace(*tier.StripePriceID) == "" {
		return "", fmt.Errorf("accommodation %q has no stripe_price_id configured", code)
	}
	return strings.TrimSpace(*tier.StripePriceID), nil
}

func (s *Service) computeCharge(ctx context.Context, req domain.SubmitRequest, mode string) (charge, error) {
	deposit, err := s.prices.GetPrice(ctx, domain.PriceDeposit)
	if err != nil {
		return charge{}, fmt.Errorf("lookup deposit price: %w", err)
	}
	currency := deposit.Currency
	if currency == "" {
		currency = "GBP"
	}

	if mode != domain.PaymentModeFull {
		paying := depositPayingCount(req)
		total := deposit.AmountPence * paying
		var lines []CheckoutLine
		if total > 0 {
			lines = []CheckoutLine{{
				Description: fmt.Sprintf("PC Summer Camp 2026 non-refundable deposit (%d camper%s)", paying, pluralS(paying)),
				AmountPence: int64(total),
				Quantity:    1,
			}}
		}
		return charge{totalPence: total, currency: currency, lines: lines}, nil
	}

	if s.stripeAmounts == nil {
		return charge{}, fmt.Errorf("stripe price catalog not configured")
	}

	var lines []CheckoutLine
	totalPence := 0

	paying := depositPayingCount(req)
	if paying > 0 && deposit.AmountPence > 0 {
		lines = append(lines, CheckoutLine{
			Description: "PC Summer Camp 2026 non-refundable deposit",
			AmountPence: int64(deposit.AmountPence),
			Quantity:    int64(paying),
		})
		totalPence += deposit.AmountPence * paying
	}

	tiers, err := s.repo.ListAccommodationTypes(ctx)
	if err != nil {
		return charge{}, fmt.Errorf("list accommodation types: %w", err)
	}
	tierByCode := map[string]domain.AccommodationType{}
	for _, t := range tiers {
		tierByCode[t.Code] = t
	}

	// Aggregate accommodation lines by (priceID, description).
	type accKey struct {
		priceID     string
		description string
		unitPence   int64
	}
	accCounts := map[accKey]int64{}

	for _, c := range req.Campers {
		if c.Attendance.Type != domain.AttendanceFullWeek {
			continue
		}
		code := strings.TrimSpace(c.Attendance.AccommodationFirstChoice)
		if code == "" {
			return charge{}, fmt.Errorf("full-week camper missing accommodation first choice")
		}
		tier := tierByCode[code]
		priceID, err := AccommodationPriceID(code, c.Age, &tier, s.cfg.StripePriceChildUnder3)
		if err != nil {
			return charge{}, err
		}
		unit, cur, err := s.stripeAmounts.Amount(ctx, priceID)
		if err != nil {
			return charge{}, fmt.Errorf("lookup price for %s: %w", code, err)
		}
		if err := ensureCurrency(currency, cur); err != nil {
			return charge{}, err
		}
		if unit <= 0 {
			continue
		}
		desc := fmt.Sprintf("%s — full week", tier.DisplayName)
		k := accKey{priceID: priceID, description: desc, unitPence: unit}
		accCounts[k]++
	}

	for k, qty := range accCounts {
		lines = append(lines, CheckoutLine{
			Description: k.description,
			AmountPence: k.unitPence,
			Quantity:    qty,
		})
		totalPence += int(k.unitPence * qty)
	}

	dayPassDays := 0
	for _, c := range req.Campers {
		if c.Attendance.Type == domain.AttendanceDayPass {
			dayPassDays += len(c.Attendance.Days)
		}
	}
	if dayPassDays > 0 {
		if s.cfg.StripePriceDayPass == "" {
			return charge{}, fmt.Errorf("STRIPE_PRICE_DAY_PASS is not configured")
		}
		unit, cur, err := s.stripeAmounts.Amount(ctx, s.cfg.StripePriceDayPass)
		if err != nil {
			return charge{}, fmt.Errorf("lookup day pass price: %w", err)
		}
		if err := ensureCurrency(currency, cur); err != nil {
			return charge{}, err
		}
		if unit > 0 {
			lines = append(lines, CheckoutLine{
				Description: "PC Summer Camp 2026 day pass",
				AmountPence: unit,
				Quantity:    int64(dayPassDays),
			})
			totalPence += int(unit * int64(dayPassDays))
		}
	}

	coachCount := coachEligibleCountFromRequest(req)
	if coachCount > 0 {
		if s.cfg.StripePriceCoach == "" {
			return charge{}, fmt.Errorf("STRIPE_PRICE_COACH is not configured")
		}
		unit, cur, err := s.stripeAmounts.Amount(ctx, s.cfg.StripePriceCoach)
		if err != nil {
			return charge{}, fmt.Errorf("lookup coach price: %w", err)
		}
		if err := ensureCurrency(currency, cur); err != nil {
			return charge{}, err
		}
		if unit > 0 {
			lines = append(lines, CheckoutLine{
				Description: "PC Summer Camp 2026 coach travel",
				AmountPence: unit,
				Quantity:    coachCount,
			})
			totalPence += int(unit * coachCount)
		}
	}

	return charge{totalPence: totalPence, currency: currency, lines: lines}, nil
}

func coachEligibleCountFromRequest(req domain.SubmitRequest) int64 {
	var n int64
	for _, c := range req.Campers {
		if c.Attendance.Type != domain.AttendanceFullWeek {
			continue
		}
		if c.Attendance.NeedsCoach != nil && *c.Attendance.NeedsCoach && c.Age >= MinDepositAge {
			n++
		}
	}
	return n
}

func ensureCurrency(expected, got string) error {
	if strings.EqualFold(expected, got) {
		return nil
	}
	return fmt.Errorf("price currency mismatch: expected %s, got %s", expected, got)
}

func (s *Service) PricingSnapshot(ctx context.Context) (*PricingSnapshot, error) {
	mode, err := s.camp.RegistrationPaymentMode(ctx)
	if err != nil {
		return nil, commonerrors.Internal(err.Error())
	}

	deposit, err := s.prices.GetPrice(ctx, domain.PriceDeposit)
	if err != nil {
		return nil, commonerrors.Internal(err.Error())
	}
	currency := deposit.Currency
	if currency == "" {
		currency = "GBP"
	}

	snap := &PricingSnapshot{
		Mode:               mode,
		Currency:           currency,
		DepositAmountPence: deposit.AmountPence,
	}

	if mode != domain.PaymentModeFull {
		return snap, nil
	}

	if s.stripeAmounts == nil {
		return nil, commonerrors.Internal("stripe price catalog not configured")
	}

	tiers, err := s.repo.ListAccommodationTypes(ctx)
	if err != nil {
		return nil, commonerrors.Internal(err.Error())
	}

	for _, t := range tiers {
		if !t.AvailableForRegistration && t.Code != AccommodationChild {
			continue
		}
		priceID, err := AccommodationPriceID(t.Code, MinDepositAge, &t, s.cfg.StripePriceChildUnder3)
		if err != nil {
			continue
		}
		amount, cur, err := s.stripeAmounts.Amount(ctx, priceID)
		if err != nil {
			return nil, commonerrors.Internal(err.Error())
		}
		if err := ensureCurrency(currency, cur); err != nil {
			return nil, commonerrors.Internal(err.Error())
		}
		snap.AccommodationTiers = append(snap.AccommodationTiers, PricingTier{
			Code:        t.Code,
			DisplayName: t.DisplayName,
			AmountPence: int(amount),
		})
	}

	if s.cfg.StripePriceChildUnder3 != "" {
		amount, cur, err := s.stripeAmounts.Amount(ctx, s.cfg.StripePriceChildUnder3)
		if err != nil {
			return nil, commonerrors.Internal(err.Error())
		}
		if err := ensureCurrency(currency, cur); err != nil {
			return nil, commonerrors.Internal(err.Error())
		}
		snap.ChildUnder3AmountPence = int(amount)
	}

	if s.cfg.StripePriceDayPass != "" {
		amount, cur, err := s.stripeAmounts.Amount(ctx, s.cfg.StripePriceDayPass)
		if err != nil {
			return nil, commonerrors.Internal(err.Error())
		}
		if err := ensureCurrency(currency, cur); err != nil {
			return nil, commonerrors.Internal(err.Error())
		}
		snap.DayPassAmountPence = int(amount)
	}

	if s.cfg.StripePriceCoach != "" {
		amount, cur, err := s.stripeAmounts.Amount(ctx, s.cfg.StripePriceCoach)
		if err != nil {
			return nil, commonerrors.Internal(err.Error())
		}
		if err := ensureCurrency(currency, cur); err != nil {
			return nil, commonerrors.Internal(err.Error())
		}
		snap.CoachAmountPence = int(amount)
	}

	return snap, nil
}

func (s *Service) ValidateFullPaymentPricing(ctx context.Context) error {
	deposit, err := s.prices.GetPrice(ctx, domain.PriceDeposit)
	if err != nil {
		return commonerrors.Internal(err.Error())
	}
	currency := deposit.Currency
	if currency == "" {
		currency = "GBP"
	}

	if s.stripeAmounts == nil {
		return commonerrors.BadRequest("stripe price catalog not configured", nil)
	}

	var problems []string

	checkPrice := func(label, priceID string) {
		if priceID == "" {
			problems = append(problems, label+" is not configured")
			return
		}
		amount, cur, err := s.stripeAmounts.Amount(ctx, priceID)
		if err != nil {
			problems = append(problems, fmt.Sprintf("%s: %v", label, err))
			return
		}
		if amount < 0 {
			problems = append(problems, fmt.Sprintf("%s has invalid amount", label))
		}
		if !strings.EqualFold(currency, cur) {
			problems = append(problems, fmt.Sprintf("%s currency is %s, expected %s", label, cur, currency))
		}
	}

	tiers, err := s.repo.ListAccommodationTypes(ctx)
	if err != nil {
		return commonerrors.Internal(err.Error())
	}

	for _, t := range tiers {
		if !t.AvailableForRegistration && t.Code != AccommodationChild {
			continue
		}
		priceID, err := AccommodationPriceID(t.Code, MinDepositAge, &t, s.cfg.StripePriceChildUnder3)
		if err != nil {
			problems = append(problems, fmt.Sprintf("%s: %v", t.DisplayName, err))
			continue
		}
		checkPrice(t.DisplayName, priceID)
	}

	checkPrice("Child under 3 (STRIPE_PRICE_CHILD_UNDER3)", s.cfg.StripePriceChildUnder3)
	checkPrice("Day pass (STRIPE_PRICE_DAY_PASS)", s.cfg.StripePriceDayPass)
	checkPrice("Coach (STRIPE_PRICE_COACH)", s.cfg.StripePriceCoach)

	if len(problems) > 0 {
		return commonerrors.BadRequest("cannot enable full payment mode: "+strings.Join(problems, "; "), nil)
	}
	return nil
}
