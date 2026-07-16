package main

import (
	"context"
	"testing"

	"pagasacentre/backend/internal/config"
	"pagasacentre/backend/internal/registration/storage"
	"pagasacentre/backend/internal/testhelper"
)

func TestApplyStripePriceOverrides_caravanOverflowUsesTentPrice(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	repo := storage.NewRepository(pool)

	tentPrice := "price_test_tent_shared"
	applyStripePriceOverrides(ctx, repo, config.Config{StripePriceTent: tentPrice})

	tent, err := repo.GetAccommodationType(ctx, "tent")
	if err != nil || tent == nil {
		t.Fatalf("get tent tier: err=%v type=%v", err, tent)
	}
	overflow, err := repo.GetAccommodationType(ctx, "caravan_overflow")
	if err != nil || overflow == nil {
		t.Fatalf("get caravan_overflow tier: err=%v type=%v", err, overflow)
	}
	if tent.StripePriceID == nil || *tent.StripePriceID != tentPrice {
		t.Fatalf("tent stripe_price_id = %v, want %q", tent.StripePriceID, tentPrice)
	}
	if overflow.StripePriceID == nil || *overflow.StripePriceID != tentPrice {
		t.Fatalf("caravan_overflow stripe_price_id = %v, want %q (same as tent)", overflow.StripePriceID, tentPrice)
	}
	if overflow.Capacity == nil || *overflow.Capacity != 16 {
		t.Fatalf("caravan_overflow capacity = %v, want 16", overflow.Capacity)
	}
	if !overflow.AvailableForRegistration {
		t.Fatal("caravan_overflow should be available for registration by default")
	}
}

func TestApplyStripePriceOverrides_caravanOverflowListedInAdminTypes(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	repo := storage.NewRepository(pool)

	types, err := repo.ListAccommodationTypes(ctx)
	if err != nil {
		t.Fatalf("ListAccommodationTypes: %v", err)
	}
	var found bool
	for _, accom := range types {
		if accom.Code == "caravan_overflow" {
			found = true
			if accom.DisplayName != "Caravan - Overflow" {
				t.Fatalf("display_name = %q", accom.DisplayName)
			}
			break
		}
	}
	if !found {
		t.Fatal("caravan_overflow not in ListAccommodationTypes")
	}
}
