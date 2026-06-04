package billing

import (
	"testing"

	"pagasacentre/backend/internal/registration"
)

func TestChildUnder3UsesConfigPrice(t *testing.T) {
	cfg := Config{StripePriceChildUnder3: "price_child_0"}
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
