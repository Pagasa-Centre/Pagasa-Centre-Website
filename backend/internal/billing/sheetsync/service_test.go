package sheetsync_test

import (
	"context"
	"testing"

	"pagasacentre/backend/internal/billing"
	"pagasacentre/backend/internal/billing/billingtest"
	"pagasacentre/backend/internal/registration/domain"
	"pagasacentre/backend/internal/testhelper"
)

func TestResyncSheet_paidGroup(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	rec := &billingtest.RecordingSheets{}
	cfg := billing.Config{}
	env := billingtest.NewEnv(t, pool, &billingtest.StubStripe{}, nil, rec, cfg)
	repo := env.Repo

	groupID, camperID := billingtest.InsertDayPassGroup(ctx, t, pool, domain.BillingNone)
	_, err := pool.Exec(ctx, `
		UPDATE registrations SET shirt_size = $2, day_pass_tshirt_option = 'tshirt_only'
		WHERE id = $1`, camperID, "adult_m")
	if err != nil {
		t.Fatalf("update camper: %v", err)
	}

	if err := env.Sheets.ResyncSheet(ctx, groupID, "Admin", domain.SkipVersionCheck); err != nil {
		t.Fatalf("ResyncSheet: %v", err)
	}
	if rec.RemoveCalls != 1 {
		t.Fatalf("remove calls = %d, want 1", rec.RemoveCalls)
	}
	if rec.AppendCalls != 1 {
		t.Fatalf("append calls = %d, want 1", rec.AppendCalls)
	}
	if rec.PendingCalls != 0 {
		t.Fatalf("pending calls = %d, want 0", rec.PendingCalls)
	}
	if len(rec.LastRows) != 1 {
		t.Fatalf("rows = %d, want 1", len(rec.LastRows))
	}
	if rec.LastRows[0].ShirtSize == nil || *rec.LastRows[0].ShirtSize != "adult_m" {
		t.Fatalf("shirt_size = %v", rec.LastRows[0].ShirtSize)
	}
	g, _ := repo.FindGroupByID(ctx, groupID)
	if g == nil || g.LastAction == nil || *g.LastAction != "sheet_resynced" {
		t.Fatalf("last_action = %v", g.LastAction)
	}
}
