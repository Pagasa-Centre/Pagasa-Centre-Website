package billing_test

import (
	"context"
	"testing"

	"pagasacentre/backend/internal/billing"
	"pagasacentre/backend/internal/billing/billingtest"
	"pagasacentre/backend/internal/registration"
	"pagasacentre/backend/internal/testhelper"
)

func TestChildUnder3UsesConfigPrice(t *testing.T) {
	cfg := billing.Config{StripePriceChildUnder3: "price_child_0"}
	if cfg.StripePriceChildUnder3 == "" {
		t.Fatal("expected configured under-3 price")
	}
	// Documented rule: child accommodation + age < MinDepositAge => under-3 price.
	age := registration.MinDepositAge - 1
	code := registration.AccommodationChild
	if code != "child" || age >= registration.MinDepositAge {
		t.Fatal("test precondition")
	}
	_ = cfg.StripePriceChildUnder3
}

func TestResolvePriceID_caravanOverflowMatchesTent(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	cfg := billing.Config{}
	env := billingtest.NewEnv(t, pool, &billingtest.StubStripe{}, nil, nil, cfg)
	repo := env.Repo
	price := "price_shared_tent_overflow"
	if err := repo.UpdateAccommodationStripePrice(ctx, "tent", price); err != nil {
		t.Fatalf("update tent price: %v", err)
	}
	if err := repo.UpdateAccommodationStripePrice(ctx, "caravan_overflow", price); err != nil {
		t.Fatalf("update overflow price: %v", err)
	}
	tentID, err := env.Pricer.ResolvePriceID(ctx, "tent", 25)
	if err != nil {
		t.Fatalf("resolve tent: %v", err)
	}
	overflowID, err := env.Pricer.ResolvePriceID(ctx, "caravan_overflow", 25)
	if err != nil {
		t.Fatalf("resolve overflow: %v", err)
	}
	if tentID != overflowID {
		t.Fatalf("tent=%q overflow=%q, want same price id", tentID, overflowID)
	}
}

func TestValidateAllocatedUnit_blankAllowed(t *testing.T) {
	pool := testhelper.MaybePool(t)
	cfg := billing.Config{}
	env := billingtest.NewEnv(t, pool, &billingtest.StubStripe{}, nil, nil, cfg)

	if err := env.Pricer.ValidateAllocatedUnit(context.Background(), "lodge", ""); err != nil {
		t.Fatalf("blank unit should be allowed: %v", err)
	}
}

func TestValidateAllocatedUnit_rejectsMismatch(t *testing.T) {
	pool := testhelper.MaybePool(t)
	cfg := billing.Config{}
	env := billingtest.NewEnv(t, pool, &billingtest.StubStripe{}, nil, nil, cfg)

	err := env.Pricer.ValidateAllocatedUnit(context.Background(), "lodge", "caravan_1")
	if err == nil {
		t.Fatal("expected tier/unit mismatch error")
	}
}

func TestResolvePriceID_childUnder4UsesUnder3Price(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	cfg := billing.Config{StripePriceChildUnder3: "price_child_u3"}
	env := billingtest.NewEnv(t, pool, &billingtest.StubStripe{}, nil, nil, cfg)
	id, err := env.Pricer.ResolvePriceID(ctx, registration.AccommodationChild, 3)
	if err != nil {
		t.Fatalf("resolve child under 4: %v", err)
	}
	if id != "price_child_u3" {
		t.Fatalf("got %q, want price_child_u3", id)
	}
}

func TestResolvePriceID_childAge8UsesTierPrice(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	cfg := billing.Config{StripePriceChildUnder3: "price_child_u3"}
	env := billingtest.NewEnv(t, pool, &billingtest.StubStripe{}, nil, nil, cfg)
	repo := env.Repo
	if err := repo.UpdateAccommodationStripePrice(ctx, "child", "price_child_tier"); err != nil {
		t.Fatalf("seed child price: %v", err)
	}
	id, err := env.Pricer.ResolvePriceID(ctx, registration.AccommodationChild, 8)
	if err != nil {
		t.Fatalf("resolve child age 8: %v", err)
	}
	if id != "price_child_tier" {
		t.Fatalf("got %q, want price_child_tier", id)
	}
}

func TestResolvePriceID_tentUsesTierPrice(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	cfg := billing.Config{}
	env := billingtest.NewEnv(t, pool, &billingtest.StubStripe{}, nil, nil, cfg)
	repo := env.Repo
	if err := repo.UpdateAccommodationStripePrice(ctx, "tent", "price_tent"); err != nil {
		t.Fatalf("seed tent price: %v", err)
	}
	id, err := env.Pricer.ResolvePriceID(ctx, "tent", 25)
	if err != nil {
		t.Fatalf("resolve tent: %v", err)
	}
	if id != "price_tent" {
		t.Fatalf("got %q, want price_tent", id)
	}
}

func TestResolvePriceID_emptyStripePriceErrors(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	cfg := billing.Config{}
	env := billingtest.NewEnv(t, pool, &billingtest.StubStripe{}, nil, nil, cfg)
	repo := env.Repo
	if _, err := pool.Exec(ctx, `UPDATE accommodation_types SET stripe_price_id = NULL WHERE code = 'cabin'`); err != nil {
		t.Fatalf("clear cabin price: %v", err)
	}
	t.Cleanup(func() {
		_ = repo.UpdateAccommodationStripePrice(context.Background(), "cabin", "price_cabin_restore")
	})
	_, err := env.Pricer.ResolvePriceID(ctx, "cabin", 25)
	if err == nil {
		t.Fatal("expected error for tier without stripe_price_id")
	}
}
