package billing

import (
	"context"
	"fmt"
	"log"
	"strings"

	"pagasacentre/backend/internal/registration/domain"
	"pagasacentre/backend/pkg/commonlibrary/apperr"
)

type accommodationReader interface {
	GetAccommodationType(ctx context.Context, code string) (*domain.AccommodationType, error)
	GetAccommodationUnit(ctx context.Context, code string) (*domain.AccommodationUnit, error)
	ListAccommodationTypes(ctx context.Context) ([]domain.AccommodationType, error)
	ListAccommodationUnits(ctx context.Context) ([]domain.AccommodationUnit, error)
}

// Pricer resolves Stripe prices and accommodation display names for billing groups.
type Pricer struct {
	read   accommodationReader
	cfg    Config
	prices PriceLookup
}

type PriceLookup interface {
	GetPrice(ctx context.Context, code string) (domain.PriceRow, error)
}

func NewPricer(read accommodationReader, cfg Config, prices PriceLookup) *Pricer {
	return &Pricer{read: read, cfg: cfg, prices: prices}
}

func (p *Pricer) DepositAmountPence(ctx context.Context) int {
	if p.prices != nil {
		if amt, err := p.prices.GetPrice(ctx, domain.PriceDeposit); err == nil && amt.AmountPence > 0 {
			return amt.AmountPence
		} else if err != nil {
			log.Printf("billing: live deposit price lookup: %v", err)
		}
	}
	return p.cfg.DepositPricePence
}

func (p *Pricer) ValidateAllocatedUnit(ctx context.Context, tierCode, unitCode string) error {
	if unitCode == "" {
		return nil
	}
	u, err := p.read.GetAccommodationUnit(ctx, unitCode)
	if err != nil {
		return fmt.Errorf("operation: %w", err)
	}
	if u == nil {
		return apperr.Invalid("unknown unit " + unitCode)
	}
	if u.AccommodationCode != tierCode {
		return apperr.Invalidf("unit %q belongs to %q, not %q", unitCode, u.AccommodationCode, tierCode)
	}
	return nil
}

func (p *Pricer) ResolvePriceID(ctx context.Context, accommodationCode string, age int) (string, error) {
	t, err := p.read.GetAccommodationType(ctx, accommodationCode)
	if err != nil {
		return "", fmt.Errorf("get accommodation type: %w", err)
	}
	priceID, err := domain.AccommodationPriceID(accommodationCode, age, t, p.cfg.ChildUnder3PriceRef)
	if err != nil {
		return "", apperr.Invalid(err.Error())
	}
	return priceID, nil
}

func (p *Pricer) PriceFor(ctx context.Context, accommodationCode string, age int, isFree bool) (string, error) {
	code := strings.TrimSpace(accommodationCode)
	if code == "" || isFree {
		return "", nil
	}
	return p.ResolvePriceID(ctx, code, age)
}

func (p *Pricer) AccommodationNames(ctx context.Context) map[string]string {
	m := map[string]string{}
	types, err := p.read.ListAccommodationTypes(ctx)
	if err != nil {
		log.Printf("billing: load accommodation names: %v", err)
		return m
	}
	for _, t := range types {
		m[t.Code] = t.DisplayName
	}
	return m
}

func (p *Pricer) UnitNames(ctx context.Context) map[string]string {
	m := map[string]string{}
	units, err := p.read.ListAccommodationUnits(ctx)
	if err != nil {
		log.Printf("billing: load unit names: %v", err)
		return m
	}
	for _, u := range units {
		m[u.Code] = u.DisplayName
	}
	return m
}
