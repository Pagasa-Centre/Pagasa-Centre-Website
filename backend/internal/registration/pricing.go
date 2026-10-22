package registration

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"pagasacentre/backend/internal/registration/domain"
	"pagasacentre/backend/pkg/commonlibrary/apperr"
)

type PriceAmounts interface {
	Amount(ctx context.Context, priceID string) (int64, string, error)
}

type Config struct {
	ChildUnder3PriceRef string
	DayPassPriceRef     string
	CoachPriceRef       string
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
	Code        string
	DisplayName string
	AmountPence int
}

// PricingSnapshot is the public price table for the registration form.
type PricingSnapshot struct {
	Mode                   string
	Currency               string
	DepositAmountPence     int
	AccommodationTiers     []PricingTier
	ChildUnder3AmountPence int
	DayPassAmountPence     int
	CoachAmountPence       int
}

func (s *Service) computeCharge(ctx context.Context, req domain.Submission, mode string) (charge, error) {
	deposit, err := s.prices.GetPrice(ctx, domain.PriceDeposit)
	if err != nil {
		return charge{}, fmt.Errorf("lookup deposit price: %w", err)
	}
	lookup := func(priceRef string) (int64, string, error) {
		return s.priceAmounts.Amount(ctx, priceRef)
	}
	in := domain.ChargeInputs{
		Mode:                mode,
		Deposit:             deposit,
		HasCatalog:          s.priceAmounts != nil,
		ChildUnder3PriceRef: s.cfg.ChildUnder3PriceRef,
		DayPassPriceRef:     s.cfg.DayPassPriceRef,
		CoachPriceRef:       s.cfg.CoachPriceRef,
	}
	if mode == domain.PaymentModeFull {
		tiers, err := s.repo.ListAccommodationTypes(ctx)
		if err != nil {
			return charge{}, fmt.Errorf("list accommodation types: %w", err)
		}
		in.Tiers = tiers
	}
	ch, err := domain.ComputeCharge(req, in, lookup)
	if err != nil {
		return charge{}, err
	}
	out := charge{currency: ch.Currency, totalPence: ch.TotalPence}
	for _, ln := range ch.Lines {
		out.lines = append(out.lines, CheckoutLine{
			Description: ln.Description,
			AmountPence: ln.AmountPence,
			Quantity:    ln.Quantity,
		})
	}
	return out, nil
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
		return nil, errors.New(err.Error())
	}

	deposit, err := s.prices.GetPrice(ctx, domain.PriceDeposit)
	if err != nil {
		return nil, errors.New(err.Error())
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

	if s.priceAmounts == nil {
		return nil, errors.New("stripe price catalog not configured")
	}

	tiers, err := s.repo.ListAccommodationTypes(ctx)
	if err != nil {
		return nil, errors.New(err.Error())
	}

	for _, t := range tiers {
		if !t.AvailableForRegistration && t.Code != domain.AccommodationChild {
			continue
		}
		priceID, err := domain.AccommodationPriceID(t.Code, domain.MinDepositAge, &t, s.cfg.ChildUnder3PriceRef)
		if err != nil {
			continue
		}
		amount, cur, err := s.priceAmounts.Amount(ctx, priceID)
		if err != nil {
			return nil, errors.New(err.Error())
		}
		if err := ensureCurrency(currency, cur); err != nil {
			return nil, errors.New(err.Error())
		}
		snap.AccommodationTiers = append(snap.AccommodationTiers, PricingTier{
			Code:        t.Code,
			DisplayName: t.DisplayName,
			AmountPence: int(amount),
		})
	}

	if s.cfg.ChildUnder3PriceRef != "" {
		amount, cur, err := s.priceAmounts.Amount(ctx, s.cfg.ChildUnder3PriceRef)
		if err != nil {
			return nil, errors.New(err.Error())
		}
		if err := ensureCurrency(currency, cur); err != nil {
			return nil, errors.New(err.Error())
		}
		snap.ChildUnder3AmountPence = int(amount)
	}

	if s.cfg.DayPassPriceRef != "" {
		amount, cur, err := s.priceAmounts.Amount(ctx, s.cfg.DayPassPriceRef)
		if err != nil {
			return nil, errors.New(err.Error())
		}
		if err := ensureCurrency(currency, cur); err != nil {
			return nil, errors.New(err.Error())
		}
		snap.DayPassAmountPence = int(amount)
	}

	if s.cfg.CoachPriceRef != "" {
		amount, cur, err := s.priceAmounts.Amount(ctx, s.cfg.CoachPriceRef)
		if err != nil {
			return nil, errors.New(err.Error())
		}
		if err := ensureCurrency(currency, cur); err != nil {
			return nil, errors.New(err.Error())
		}
		snap.CoachAmountPence = int(amount)
	}

	return snap, nil
}

func (s *Service) ValidateFullPaymentPricing(ctx context.Context) error {
	deposit, err := s.prices.GetPrice(ctx, domain.PriceDeposit)
	if err != nil {
		return fmt.Errorf("pricing: %w", err)
	}
	currency := deposit.Currency
	if currency == "" {
		currency = "GBP"
	}

	if s.priceAmounts == nil {
		return apperr.Invalid("stripe price catalog not configured")
	}

	var problems []string

	checkPrice := func(label, priceID string) {
		if priceID == "" {
			problems = append(problems, label+" is not configured")
			return
		}
		amount, cur, err := s.priceAmounts.Amount(ctx, priceID)
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
		return fmt.Errorf("pricing: %w", err)
	}

	for _, t := range tiers {
		if !t.AvailableForRegistration && t.Code != domain.AccommodationChild {
			continue
		}
		priceID, err := domain.AccommodationPriceID(t.Code, domain.MinDepositAge, &t, s.cfg.ChildUnder3PriceRef)
		if err != nil {
			problems = append(problems, fmt.Sprintf("%s: %v", t.DisplayName, err))
			continue
		}
		checkPrice(t.DisplayName, priceID)
	}

	checkPrice("Child under 3 (STRIPE_PRICE_CHILD_UNDER3)", s.cfg.ChildUnder3PriceRef)
	checkPrice("Day pass (STRIPE_PRICE_DAY_PASS)", s.cfg.DayPassPriceRef)
	checkPrice("Coach (STRIPE_PRICE_COACH)", s.cfg.CoachPriceRef)

	if len(problems) > 0 {
		return apperr.Invalid("cannot enable full payment mode: " + strings.Join(problems, "; "))
	}
	return nil
}
