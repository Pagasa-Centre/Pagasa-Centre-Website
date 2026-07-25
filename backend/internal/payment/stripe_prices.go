package payment

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/stripe/stripe-go/v85"
	"github.com/stripe/stripe-go/v85/price"
)

type cachedPrice struct {
	amountPence int64
	currency    string
	expiresAt   time.Time
}

// PriceCatalog reads flat unit amounts off Stripe Price objects, cached in
// process for 60s. Only flat unit_amount prices are usable because both the
// form total and the Checkout line amount are derived from this number.
type PriceCatalog struct {
	mu    sync.RWMutex
	ttl   time.Duration
	cache map[string]cachedPrice
}

func NewPriceCatalog() *PriceCatalog {
	return &PriceCatalog{
		ttl:   60 * time.Second,
		cache: make(map[string]cachedPrice),
	}
}

// Amount returns the flat unit amount in pence and currency for a Stripe Price.
func (c *PriceCatalog) Amount(ctx context.Context, priceID string) (pence int64, currency string, err error) {
	if priceID == "" {
		return 0, "", fmt.Errorf("price id is empty")
	}

	c.mu.RLock()
	if entry, ok := c.cache[priceID]; ok && time.Now().Before(entry.expiresAt) {
		c.mu.RUnlock()
		return entry.amountPence, entry.currency, nil
	}
	c.mu.RUnlock()

	params := &stripe.PriceParams{}
	params.Context = ctx
	p, err := price.Get(priceID, params)
	if err != nil {
		return 0, "", fmt.Errorf("get stripe price %s: %w", priceID, err)
	}
	if !p.Active {
		return 0, "", fmt.Errorf("stripe price %s is inactive", priceID)
	}
	if p.BillingScheme == stripe.PriceBillingSchemeTiered {
		return 0, "", fmt.Errorf("stripe price %s has no flat unit_amount (tiered/metered)", priceID)
	}

	amount := p.UnitAmount
	cur := string(p.Currency)

	c.mu.Lock()
	c.cache[priceID] = cachedPrice{
		amountPence: amount,
		currency:    cur,
		expiresAt:   time.Now().Add(c.ttl),
	}
	c.mu.Unlock()

	return amount, cur, nil
}
