package invoicing_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"pagasacentre/backend/internal/billing"
	"pagasacentre/backend/internal/billing/billingtest"
	"pagasacentre/backend/internal/billing/camperedit"
	invstorage "pagasacentre/backend/internal/billing/invoicing/storage"
	"pagasacentre/backend/internal/registration"
	"pagasacentre/backend/internal/registration/domain"
	"pagasacentre/backend/internal/testhelper"
	"pagasacentre/backend/pkg/commonlibrary/apperr"
)

func strPtr(s string) *string { return &s }

type mutablePrices struct{ depositPence int }

type fakePrices struct {
	depositPence int
	err          error
}

func camperByID(t *testing.T, campers []domain.Camper, id string) domain.Camper {
	t.Helper()
	for _, c := range campers {
		if c.ID == id {
			return c
		}
	}
	t.Fatalf("camper %s not found", id)
	return domain.Camper{}
}

func editFor(c domain.Camper) camperedit.EditCamperRequest {
	return camperedit.EditCamperRequest{
		FirstName:                  c.FirstName,
		LastName:                   c.LastName,
		Gender:                     c.Gender,
		Age:                        c.Age,
		CellLeaderName:             c.CellLeaderName,
		IsCellLeader:               c.IsCellLeader,
		ShirtSize:                  billing.StrVal(c.ShirtSize),
		DietaryRequirements:        billing.StrVal(c.DietaryRequirements),
		AccommodationFirstChoice:   billing.StrVal(c.AccommodationFirstChoice),
		AccommodationSecondChoice:  billing.StrVal(c.AccommodationSecondChoice),
		AllocatedAccommodationCode: billing.StrVal(c.AllocatedAccommodationCode),
		AllocatedUnitCode:          billing.StrVal(c.AllocatedUnitCode),
	}
}

func (f fakePrices) GetPrice(_ context.Context, code string) (registration.PriceRow, error) {
	if f.err != nil {
		return registration.PriceRow{}, f.err
	}
	if code != domain.PriceDeposit {
		return registration.PriceRow{}, errors.New("unexpected price code " + code)
	}
	return registration.PriceRow{AmountPence: f.depositPence, Currency: "GBP"}, nil
}

func (p *mutablePrices) GetPrice(_ context.Context, code string) (registration.PriceRow, error) {
	if code != domain.PriceDeposit {
		return registration.PriceRow{}, errors.New("unexpected price code " + code)
	}
	return registration.PriceRow{AmountPence: p.depositPence, Currency: "GBP"}, nil
}

func TestDeleteRegistration_unpaid(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	stub := &billingtest.StubStripe{}
	cfg := billing.Config{}
	env := billingtest.NewEnv(t, pool, stub, nil, nil, cfg)
	repo := env.Repo

	var groupID, camperID string
	err := pool.QueryRow(ctx, `
		INSERT INTO registration_groups (
			contact_first_name, contact_last_name, contact_email, contact_phone,
			payment_status, total_amount_pence, currency, billing_status
		) VALUES ('Day', 'Pass', 'day@example.com', '07000000000', 'pending', 0, 'GBP', 'none')
		RETURNING id`).Scan(&groupID)
	if err != nil {
		t.Fatalf("insert group: %v", err)
	}
	err = pool.QueryRow(ctx, `
		INSERT INTO registrations (
			group_id, is_main_contact, first_name, last_name, gender, age,
			cell_leader_name, is_cell_leader, attendance_type
		) VALUES ($1, true, 'Alex', 'Day', 'male', 30, 'Leader', false, 'day_pass')
		RETURNING id`, groupID).Scan(&camperID)
	if err != nil {
		t.Fatalf("insert camper: %v", err)
	}

	sum, err := env.Invoicing.DeleteRegistration(ctx, groupID, "Admin", domain.SkipVersionCheck)
	if err != nil {
		t.Fatalf("DeleteRegistration: %v", err)
	}
	if sum.ContactEmail != "day@example.com" {
		t.Fatalf("contact email = %q", sum.ContactEmail)
	}
	if sum.BalanceRefunded || sum.InvoiceVoided {
		t.Fatal("expected no Stripe cleanup for unpaid group")
	}
	if sum.RetainedPence != 0 {
		t.Fatalf("retained = %d, want 0: they never paid a deposit", sum.RetainedPence)
	}
	g, err := repo.FindGroupByID(ctx, groupID)
	if err != nil {
		t.Fatalf("FindGroupByID: %v", err)
	}
	if g != nil {
		t.Fatal("group row should be gone")
	}
	campers, err := repo.CampersForGroup(ctx, groupID)
	if err != nil {
		t.Fatalf("CampersForGroup: %v", err)
	}
	if len(campers) != 0 {
		t.Fatalf("expected 0 campers, got %d", len(campers))
	}
}

func TestDeleteRegistration_paidDepositIsKeptNotRefunded(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	stub := &billingtest.StubStripe{}
	cfg := billing.Config{}
	env := billingtest.NewEnv(t, pool, stub, nil, nil, cfg)
	repo := env.Repo

	var groupID string
	err := pool.QueryRow(ctx, `
		INSERT INTO registration_groups (
			contact_first_name, contact_last_name, contact_email, contact_phone,
			payment_status, stripe_payment_intent_id, total_amount_pence, currency, billing_status
		) VALUES ('Paid', 'Deposit', 'paid@example.com', '07000000000', 'paid', 'pi_test_deposit', 5000, 'GBP', 'none')
		RETURNING id`).Scan(&groupID)
	if err != nil {
		t.Fatalf("insert group: %v", err)
	}

	sum, err := env.Invoicing.DeleteRegistration(ctx, groupID, "Admin", domain.SkipVersionCheck)
	if err != nil {
		t.Fatalf("DeleteRegistration: %v", err)
	}
	if len(stub.RefundInvCall) != 0 {
		t.Fatalf("refunds asked of Stripe = %+v, want none", stub.RefundInvCall)
	}
	if sum.RetainedPence != 5000 {
		t.Fatalf("retained = %d, want the 5000 deposit they paid", sum.RetainedPence)
	}
	if sum.AmountPence != 0 {
		t.Fatalf("refunded = %d, want 0", sum.AmountPence)
	}
	g, _ := repo.FindGroupByID(ctx, groupID)
	if g != nil {
		t.Fatal("group should be deleted")
	}
}

func TestDeleteRegistration_balancePaidRefundsBalanceOnly(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	inv := "in_test_full"
	stub := &billingtest.StubStripe{
		RefundInv: func(_ context.Context, id string, _ int64) (int64, error) {
			if id != inv {
				t.Fatalf("invoice = %q", id)
			}
			return 12000, nil
		},
	}
	cfg := billing.Config{}
	env := billingtest.NewEnv(t, pool, stub, nil, nil, cfg)
	repo := env.Repo

	var groupID string
	err := pool.QueryRow(ctx, `
		INSERT INTO registration_groups (
			contact_first_name, contact_last_name, contact_email, contact_phone,
			payment_status, stripe_payment_intent_id, stripe_invoice_id,
			total_amount_pence, currency, billing_status
		) VALUES ('Full', 'Paid', 'full@example.com', '07000000000', 'paid', 'pi_test_full', $1, 5000, 'GBP', 'balance_paid')
		RETURNING id`, inv).Scan(&groupID)
	if err != nil {
		t.Fatalf("insert group: %v", err)
	}

	sum, err := env.Invoicing.DeleteRegistration(ctx, groupID, "Admin", domain.SkipVersionCheck)
	if err != nil {
		t.Fatalf("DeleteRegistration: %v", err)
	}
	if len(stub.RefundInvCall) != 1 {
		t.Fatalf("refund calls = %+v, want exactly the balance invoice", stub.RefundInvCall)
	}
	if stub.RefundInvCall[0].KeepPence != 0 {
		t.Fatalf("kept back %d of the invoice, want 0: no deposit was billed on it",
			stub.RefundInvCall[0].KeepPence)
	}
	if sum.AmountPence != 12000 {
		t.Fatalf("refunded = %d, want the 12000 balance only", sum.AmountPence)
	}
	if sum.RetainedPence != 5000 {
		t.Fatalf("retained = %d, want the 5000 deposit", sum.RetainedPence)
	}
	if !sum.BalanceRefunded {
		t.Fatal("balance refund should be reported")
	}
	g, _ := repo.FindGroupByID(ctx, groupID)
	if g != nil {
		t.Fatal("group should be deleted")
	}
}

func TestDeleteRegistration_keepsDepositBilledOnTheInvoice(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	inv := "in_late_arrival"
	stub := &billingtest.StubStripe{
		RefundInv: func(_ context.Context, _ string, keepPence int64) (int64, error) {
			return 17000 - keepPence, nil
		},
	}
	cfg := billing.Config{}
	env := billingtest.NewEnv(t, pool, stub, nil, nil, cfg)

	var groupID string
	err := pool.QueryRow(ctx, `
		INSERT INTO registration_groups (
			contact_first_name, contact_last_name, contact_email, contact_phone,
			payment_status, stripe_payment_intent_id, stripe_invoice_id,
			total_amount_pence, currency, billing_status
		) VALUES ('Late', 'Arrival', 'late@example.com', '07000000000', 'paid', 'pi_late', $1, 10000, 'GBP', 'balance_paid')
		RETURNING id`, inv).Scan(&groupID)
	if err != nil {
		t.Fatalf("insert group: %v", err)
	}
	// Two campers paid a deposit at checkout (the group's 10000) and a third was
	// added later, paying 5000 of deposit on the invoice.
	if _, err := pool.Exec(ctx, `
		INSERT INTO registrations (
			group_id, is_main_contact, first_name, last_name, gender, age,
			cell_leader_name, is_cell_leader, attendance_type, deposit_paid_pence
		) VALUES ($1, true, 'Naomi', 'Late', 'female', 24, 'Leader', false, 'full_week', 5000)`,
		groupID); err != nil {
		t.Fatalf("insert camper: %v", err)
	}

	sum, err := env.Invoicing.DeleteRegistration(ctx, groupID, "Admin", domain.SkipVersionCheck)
	if err != nil {
		t.Fatalf("DeleteRegistration: %v", err)
	}
	if len(stub.RefundInvCall) != 1 || stub.RefundInvCall[0].KeepPence != 5000 {
		t.Fatalf("refund calls = %+v, want the invoice refunded less 5000 of deposit",
			stub.RefundInvCall)
	}
	if sum.RetainedPence != 15000 {
		t.Fatalf("retained = %d, want 15000: 10000 at checkout plus 5000 on the invoice",
			sum.RetainedPence)
	}
	if sum.AmountPence != 12000 {
		t.Fatalf("refunded = %d, want 12000: the 17000 invoice less the 5000 deposit",
			sum.AmountPence)
	}
}

func TestDeleteRegistration_refusesPaidInFullRegistration(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	stub := &billingtest.StubStripe{}
	cfg := billing.Config{}
	env := billingtest.NewEnv(t, pool, stub, nil, nil, cfg)
	repo := env.Repo

	var groupID string
	err := pool.QueryRow(ctx, `
		INSERT INTO registration_groups (
			contact_first_name, contact_last_name, contact_email, contact_phone,
			payment_status, stripe_payment_intent_id, stripe_invoice_id,
			total_amount_pence, currency, billing_status, paid_in_full_at_registration
		) VALUES ('One', 'Payment', 'one@example.com', '07000000000', 'paid', 'pi_one', 'in_one',
		          45000, 'GBP', 'balance_paid', true)
		RETURNING id`).Scan(&groupID)
	if err != nil {
		t.Fatalf("insert group: %v", err)
	}

	_, err = env.Invoicing.DeleteRegistration(ctx, groupID, "Admin", domain.SkipVersionCheck)
	var de *apperr.Error
	if !errors.As(err, &de) || de.Kind != apperr.KindInvalid {
		t.Fatalf("expected bad_request, got %v", err)
	}
	if !strings.Contains(de.Message, "Stripe") {
		t.Fatalf("message = %q, should send the admin to Stripe", de.Message)
	}
	if len(stub.RefundInvCall) != 0 {
		t.Fatalf("refunds asked of Stripe = %+v, want none", stub.RefundInvCall)
	}
	if g, _ := repo.FindGroupByID(ctx, groupID); g == nil {
		t.Fatal("registration should be left exactly as it was")
	}
}

func TestDeleteRegistration_paidInFullButNeverPaidStillDeletes(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	cfg := billing.Config{}
	env := billingtest.NewEnv(t, pool, &billingtest.StubStripe{}, nil, nil, cfg)
	repo := env.Repo

	var groupID string
	err := pool.QueryRow(ctx, `
		INSERT INTO registration_groups (
			contact_first_name, contact_last_name, contact_email, contact_phone,
			payment_status, total_amount_pence, currency, billing_status,
			paid_in_full_at_registration
		) VALUES ('Never', 'Paid', 'never@example.com', '07000000000', 'pending', 45000, 'GBP',
		          'none', true)
		RETURNING id`).Scan(&groupID)
	if err != nil {
		t.Fatalf("insert group: %v", err)
	}

	sum, err := env.Invoicing.DeleteRegistration(ctx, groupID, "Admin", domain.SkipVersionCheck)
	if err != nil {
		t.Fatalf("DeleteRegistration: %v", err)
	}
	if sum.RetainedPence != 0 {
		t.Fatalf("retained = %d, want 0: no money ever arrived", sum.RetainedPence)
	}
	if g, _ := repo.FindGroupByID(ctx, groupID); g != nil {
		t.Fatal("group should be deleted")
	}
}

func TestDeleteRegistration_sponsoredRetainsNothing(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	stub := &billingtest.StubStripe{}
	cfg := billing.Config{}
	env := billingtest.NewEnv(t, pool, stub, nil, nil, cfg)

	var groupID string
	err := pool.QueryRow(ctx, `
		INSERT INTO registration_groups (
			contact_first_name, contact_last_name, contact_email, contact_phone,
			payment_status, total_amount_pence, currency, billing_status, is_free
		) VALUES ('Spon', 'Sored', 'spon@example.com', '07000000000', 'paid', 0, 'GBP',
		          'free_confirmed', true)
		RETURNING id`).Scan(&groupID)
	if err != nil {
		t.Fatalf("insert group: %v", err)
	}

	sum, err := env.Invoicing.DeleteRegistration(ctx, groupID, "Admin", domain.SkipVersionCheck)
	if err != nil {
		t.Fatalf("DeleteRegistration: %v", err)
	}
	if sum.RetainedPence != 0 || sum.AmountPence != 0 {
		t.Fatalf("summary = %+v, want nothing kept and nothing refunded", sum)
	}
	if len(stub.RefundInvCall) != 0 {
		t.Fatalf("refunds asked of Stripe = %+v, want none", stub.RefundInvCall)
	}
}

func TestDeleteRegistration_depositCoveringWholeBalanceRefundsNothing(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	const invoicePaid = 5000
	stub := &billingtest.StubStripe{
		// Mirrors what RefundInvoiceKeeping does against real Stripe: nothing is
		// refundable once the kept deposit covers what was paid.
		RefundInv: func(_ context.Context, _ string, keepPence int64) (int64, error) {
			if refundable := int64(invoicePaid) - keepPence; refundable > 0 {
				return refundable, nil
			}
			return 0, nil
		},
	}
	cfg := billing.Config{}
	env := billingtest.NewEnv(t, pool, stub, nil, nil, cfg)
	repo := env.Repo

	var groupID string
	err := pool.QueryRow(ctx, `
		INSERT INTO registration_groups (
			contact_first_name, contact_last_name, contact_email, contact_phone,
			payment_status, stripe_payment_intent_id, stripe_invoice_id,
			total_amount_pence, currency, billing_status
		) VALUES ('All', 'Deposit', 'alldep@example.com', '07000000000', 'paid', 'pi_alldep',
		          'in_alldep', 10000, 'GBP', 'balance_paid')
		RETURNING id`).Scan(&groupID)
	if err != nil {
		t.Fatalf("insert group: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO registrations (
			group_id, is_main_contact, first_name, last_name, gender, age,
			cell_leader_name, is_cell_leader, attendance_type, deposit_paid_pence
		) VALUES ($1, true, 'Only', 'Deposit', 'male', 22, 'Leader', false, 'full_week', $2)`,
		groupID, invoicePaid); err != nil {
		t.Fatalf("insert camper: %v", err)
	}

	sum, err := env.Invoicing.DeleteRegistration(ctx, groupID, "Admin", domain.SkipVersionCheck)
	if err != nil {
		t.Fatalf("DeleteRegistration: %v", err)
	}
	if len(stub.RefundInvCall) != 1 || stub.RefundInvCall[0].KeepPence != invoicePaid {
		t.Fatalf("refund calls = %+v, want the whole %d held back", stub.RefundInvCall, invoicePaid)
	}
	if sum.AmountPence != 0 || sum.BalanceRefunded {
		t.Fatalf("summary = %+v, want nothing refunded", sum)
	}
	if sum.RetainedPence != 15000 {
		t.Fatalf("retained = %d, want 15000: 10000 at checkout plus the 5000 invoiced deposit",
			sum.RetainedPence)
	}
	if g, _ := repo.FindGroupByID(ctx, groupID); g != nil {
		t.Fatal("group should be deleted")
	}
}

func TestDeleteRegistration_voidsOpenCoachInvoice(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	cases := map[string]struct {
		coachPaid  bool
		wantVoided []string
	}{
		"unpaid coach invoice is voided": {coachPaid: false, wantVoided: []string{"in_coach"}},
		"paid coach fee is left alone":   {coachPaid: true, wantVoided: nil},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			var voided []string
			stub := &billingtest.StubStripe{
				VoidInvIdem: func(_ context.Context, id string) error {
					voided = append(voided, id)
					return nil
				},
			}
			cfg := billing.Config{}
			env := billingtest.NewEnv(t, pool, stub, nil, nil, cfg)
			repo := env.Repo

			var paidAt *time.Time
			if tc.coachPaid {
				now := time.Now().UTC()
				paidAt = &now
			}
			var groupID string
			err := pool.QueryRow(ctx, `
				INSERT INTO registration_groups (
					contact_first_name, contact_last_name, contact_email, contact_phone,
					payment_status, stripe_payment_intent_id, stripe_coach_invoice_id,
					coach_fee_paid_at, total_amount_pence, currency, billing_status
				) VALUES ('Coach', 'Open', 'coach@example.com', '07000000000', 'paid', 'pi_coach',
				          'in_coach', $1, 5000, 'GBP', 'none')
				RETURNING id`, paidAt).Scan(&groupID)
			if err != nil {
				t.Fatalf("insert group: %v", err)
			}

			if _, err := env.Invoicing.DeleteRegistration(ctx, groupID, "Admin", domain.SkipVersionCheck); err != nil {
				t.Fatalf("DeleteRegistration: %v", err)
			}
			if len(voided) != len(tc.wantVoided) {
				t.Fatalf("voided = %v, want %v", voided, tc.wantVoided)
			}
			for i, want := range tc.wantVoided {
				if voided[i] != want {
					t.Fatalf("voided = %v, want %v", voided, tc.wantVoided)
				}
			}
			if g, _ := repo.FindGroupByID(ctx, groupID); g != nil {
				t.Fatal("group should be deleted")
			}
		})
	}
}

func TestDeleteRegistration_manuallySettledBalanceDeletesWithoutRefund(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	stub := &billingtest.StubStripe{
		RefundInv: func(_ context.Context, _ string, _ int64) (int64, error) {
			return 0, nil // no payment found behind the invoice
		},
	}
	cfg := billing.Config{}
	env := billingtest.NewEnv(t, pool, stub, nil, nil, cfg)
	repo := env.Repo

	var groupID string
	err := pool.QueryRow(ctx, `
		INSERT INTO registration_groups (
			contact_first_name, contact_last_name, contact_email, contact_phone,
			payment_status, stripe_payment_intent_id, stripe_invoice_id,
			total_amount_pence, currency, billing_status
		) VALUES ('Bank', 'Transfer', 'bank@example.com', '07000000000', 'paid', 'pi_bank', 'in_bank',
		          5000, 'GBP', 'balance_paid')
		RETURNING id`).Scan(&groupID)
	if err != nil {
		t.Fatalf("insert group: %v", err)
	}

	sum, err := env.Invoicing.DeleteRegistration(ctx, groupID, "Admin", domain.SkipVersionCheck)
	if err != nil {
		t.Fatalf("DeleteRegistration: %v", err)
	}
	if sum.BalanceRefunded {
		t.Fatal("nothing was refunded, so the summary must not say it was")
	}
	if sum.RetainedPence != 5000 {
		t.Fatalf("retained = %d, want the 5000 deposit", sum.RetainedPence)
	}
	if g, _ := repo.FindGroupByID(ctx, groupID); g != nil {
		t.Fatal("group should be deleted")
	}
}

func TestDeleteRegistration_stripeFailureAborts(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	cases := map[string]struct {
		stub           *billingtest.StubStripe
		billingStatus  domain.BillingStatus
		coachInvoiceID *string
	}{
		"balance refund fails": {
			stub: &billingtest.StubStripe{
				RefundInv: func(context.Context, string, int64) (int64, error) {
					return 0, errors.New("stripe down")
				},
			},
			billingStatus: domain.BillingBalancePaid,
		},
		"invoice void fails": {
			stub: &billingtest.StubStripe{
				VoidInvIdem: func(context.Context, string) error {
					return errors.New("stripe down")
				},
			},
			billingStatus: domain.BillingInvoiced,
		},
		"coach invoice void fails": {
			stub: &billingtest.StubStripe{
				VoidInvIdem: func(context.Context, string) error {
					return errors.New("stripe down")
				},
			},
			billingStatus:  domain.BillingNone,
			coachInvoiceID: strPtr("in_coach_fail"),
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			cfg := billing.Config{}
			env := billingtest.NewEnv(t, pool, tc.stub, nil, nil, cfg)
			repo := env.Repo

			var groupID string
			err := pool.QueryRow(ctx, `
				INSERT INTO registration_groups (
					contact_first_name, contact_last_name, contact_email, contact_phone,
					payment_status, stripe_payment_intent_id, stripe_invoice_id,
					stripe_coach_invoice_id, total_amount_pence, currency, billing_status
				) VALUES ('Fail', 'Stripe', 'fail@example.com', '07000000000', 'paid', 'pi_fail',
				          'in_fail', $1, 5000, 'GBP', $2)
				RETURNING id`, tc.coachInvoiceID, tc.billingStatus).Scan(&groupID)
			if err != nil {
				t.Fatalf("insert group: %v", err)
			}

			_, err = env.Invoicing.DeleteRegistration(ctx, groupID, "Admin", domain.SkipVersionCheck)
			var de *apperr.Error
			if !errors.As(err, &de) || de.Kind != apperr.KindInvalid {
				t.Fatalf("expected bad_request, got %v", err)
			}
			if g, _ := repo.FindGroupByID(ctx, groupID); g == nil {
				t.Fatal("group should still exist after the Stripe step failed")
			}
		})
	}
}

func TestDeleteRegistration_staleVersion(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	cfg := billing.Config{}
	env := billingtest.NewEnv(t, pool, &billingtest.StubStripe{}, nil, nil, cfg)
	repo := env.Repo

	var groupID string
	err := pool.QueryRow(ctx, `
		INSERT INTO registration_groups (
			contact_first_name, contact_last_name, contact_email, contact_phone,
			payment_status, total_amount_pence, currency, billing_status, version
		) VALUES ('Stale', 'Version', 'stale@example.com', '07000000000', 'pending', 0, 'GBP', 'none', 5)
		RETURNING id`).Scan(&groupID)
	if err != nil {
		t.Fatalf("insert group: %v", err)
	}

	_, err = env.Invoicing.DeleteRegistration(ctx, groupID, "Admin", 1)
	var de *apperr.Error
	if !errors.As(err, &de) || de.Kind != apperr.KindConflict || de.Code != "stale_state" {
		t.Fatalf("expected stale_state, got %v", err)
	}
	g, _ := repo.FindGroupByID(ctx, groupID)
	if g == nil {
		t.Fatal("group should still exist")
	}
}

func TestDeleteRegistration_sheetFailureNonFatal(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	cfg := billing.Config{}
	env := billingtest.NewEnv(t, pool, &billingtest.StubStripe{}, nil, billingtest.FailingSheets{}, cfg)
	repo := env.Repo

	var groupID string
	err := pool.QueryRow(ctx, `
		INSERT INTO registration_groups (
			contact_first_name, contact_last_name, contact_email, contact_phone,
			payment_status, total_amount_pence, currency, billing_status
		) VALUES ('Sheet', 'Fail', 'sheet@example.com', '07000000000', 'pending', 0, 'GBP', 'none')
		RETURNING id`).Scan(&groupID)
	if err != nil {
		t.Fatalf("insert group: %v", err)
	}

	_, err = env.Invoicing.DeleteRegistration(ctx, groupID, "Admin", domain.SkipVersionCheck)
	if err != nil {
		t.Fatalf("DeleteRegistration should succeed despite sheet error: %v", err)
	}
	g, _ := repo.FindGroupByID(ctx, groupID)
	if g != nil {
		t.Fatal("group should be deleted")
	}
}

func TestDeleteRegistration_alreadyRefundedIdempotent(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	stub := &billingtest.StubStripe{
		RefundInv: func(context.Context, string, int64) (int64, error) {
			return 0, nil // simulates charge_already_refunded -> 0 from the helper
		},
	}
	cfg := billing.Config{}
	env := billingtest.NewEnv(t, pool, stub, nil, nil, cfg)
	repo := env.Repo

	var groupID string
	err := pool.QueryRow(ctx, `
		INSERT INTO registration_groups (
			contact_first_name, contact_last_name, contact_email, contact_phone,
			payment_status, stripe_payment_intent_id, stripe_invoice_id,
			total_amount_pence, currency, billing_status
		) VALUES ('Already', 'Refunded', 'already@example.com', '07000000000', 'paid', 'pi_done',
		          'in_done', 5000, 'GBP', 'balance_paid')
		RETURNING id`).Scan(&groupID)
	if err != nil {
		t.Fatalf("insert group: %v", err)
	}

	_, err = env.Invoicing.DeleteRegistration(ctx, groupID, "Admin", domain.SkipVersionCheck)
	if err != nil {
		t.Fatalf("DeleteRegistration: %v", err)
	}
	g, _ := repo.FindGroupByID(ctx, groupID)
	if g != nil {
		t.Fatal("group should be deleted after idempotent refund")
	}
}

func TestDeleteRegistration_invoicedVoidsAndDeletes(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	inv := "in_test_open"
	var voided bool
	stub := &billingtest.StubStripe{
		VoidInvIdem: func(_ context.Context, id string) error {
			voided = true
			if id != inv {
				t.Fatalf("void invoice = %q, want %q", id, inv)
			}
			return nil
		},
	}
	cfg := billing.Config{}
	env := billingtest.NewEnv(t, pool, stub, nil, nil, cfg)
	repo := env.Repo

	var groupID string
	err := pool.QueryRow(ctx, `
		INSERT INTO registration_groups (
			contact_first_name, contact_last_name, contact_email, contact_phone,
			payment_status, stripe_payment_intent_id, stripe_invoice_id,
			total_amount_pence, currency, billing_status
		) VALUES ('Open', 'Invoice', 'open@example.com', '07000000000', 'paid', 'pi_dep', $1, 5000, 'GBP', 'invoiced')
		RETURNING id`, inv).Scan(&groupID)
	if err != nil {
		t.Fatalf("insert group: %v", err)
	}

	sum, err := env.Invoicing.DeleteRegistration(ctx, groupID, "Admin", domain.SkipVersionCheck)
	if err != nil {
		t.Fatalf("DeleteRegistration: %v", err)
	}
	if !voided || !sum.InvoiceVoided {
		t.Fatalf("expected invoice voided, summary=%+v", sum)
	}
	// The invoice was never paid, so there is nothing to refund — and the deposit
	// they did pay is kept.
	if sum.AmountPence != 0 || len(stub.RefundInvCall) != 0 {
		t.Fatalf("refunded %d via %+v, want nothing refunded", sum.AmountPence, stub.RefundInvCall)
	}
	if sum.RetainedPence != 5000 {
		t.Fatalf("retained = %d, want the 5000 deposit", sum.RetainedPence)
	}
	g, _ := repo.FindGroupByID(ctx, groupID)
	if g != nil {
		t.Fatal("group should be deleted")
	}
}

func TestSendInvoice_foldsCoachLine(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	cfg := billing.Config{StripePriceCoach: "price_coach"}
	var captured []billing.InvoiceLine
	var capturedType string
	stub := &billingtest.StubStripe{
		EnsureCust: func(_ context.Context, existingID, _, _, _ string) (string, error) {
			return "cus_seed", nil
		},
		CreateInv: func(_ context.Context, _, _ string, lines []billing.InvoiceLine, _ int64, invoiceType string) (billing.InvoiceResult, error) {
			captured = lines
			capturedType = invoiceType
			return billing.InvoiceResult{ID: "in_balance", StripeEmailed: true}, nil
		},
	}
	env := billingtest.NewEnv(t, pool, stub, nil, nil, cfg)
	repo := env.Repo
	if err := repo.UpdateAccommodationStripePrice(ctx, "lodge", "price_lodge"); err != nil {
		t.Fatalf("set lodge price: %v", err)
	}

	groupID := billingtest.InsertCoachGroup(ctx, t, pool, domain.BillingAllocated, "lodge", false)
	if err := env.Invoicing.SendInvoice(ctx, groupID, "Admin", domain.SkipVersionCheck); err != nil {
		t.Fatalf("SendInvoice: %v", err)
	}
	if capturedType != "balance" {
		t.Fatalf("invoice type = %q, want balance", capturedType)
	}
	var haveLodge, haveCoach bool
	for _, l := range captured {
		if l.PriceID == "price_lodge" && l.Quantity == 1 {
			haveLodge = true
		}
		if l.PriceID == "price_coach" && l.Quantity == 1 {
			haveCoach = true
		}
	}
	if !haveLodge || !haveCoach {
		t.Fatalf("lines = %+v, want lodge + coach", captured)
	}
	g, _ := repo.FindGroupByID(ctx, groupID)
	if g == nil || !g.CoachIncludedInBalance {
		t.Fatalf("coach_included_in_balance not set: %+v", g)
	}
	if g.BillingStatus != domain.BillingInvoiced {
		t.Fatalf("billing_status = %q, want invoiced", g.BillingStatus)
	}
}

func TestSendCoachInvoice_paidGroup(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	var captured []billing.InvoiceLine
	var capturedType string
	stub := &billingtest.StubStripe{
		EnsureCust: func(_ context.Context, existingID, _, _, _ string) (string, error) {
			return "cus_seed", nil
		},
		CreateInv: func(_ context.Context, _, _ string, lines []billing.InvoiceLine, _ int64, invoiceType string) (billing.InvoiceResult, error) {
			captured = lines
			capturedType = invoiceType
			return billing.InvoiceResult{ID: "in_coach", StripeEmailed: true}, nil
		},
	}
	cfg := billing.Config{StripePriceCoach: "price_coach"}
	env := billingtest.NewEnv(t, pool, stub, nil, nil, cfg)
	repo := env.Repo

	groupID := billingtest.InsertCoachGroup(ctx, t, pool, domain.BillingBalancePaid, "lodge", false)
	if err := env.Invoicing.SendCoachInvoice(ctx, groupID, "Admin", domain.SkipVersionCheck); err != nil {
		t.Fatalf("SendCoachInvoice: %v", err)
	}
	if capturedType != "coach" {
		t.Fatalf("invoice type = %q, want coach", capturedType)
	}
	if len(captured) != 1 || captured[0].PriceID != "price_coach" || captured[0].Quantity != 1 {
		t.Fatalf("lines = %+v, want single coach line", captured)
	}
	g, _ := repo.FindGroupByID(ctx, groupID)
	if g == nil || g.StripeCoachInvoiceID == nil || *g.StripeCoachInvoiceID != "in_coach" {
		t.Fatalf("coach invoice id not stored: %+v", g)
	}
	if g.BillingStatus != domain.BillingBalancePaid {
		t.Fatalf("billing_status = %q, want unchanged balance_paid", g.BillingStatus)
	}

	// Second send rejects — already charged.
	err := env.Invoicing.SendCoachInvoice(ctx, groupID, "Admin", domain.SkipVersionCheck)
	var de *apperr.Error
	if !errors.As(err, &de) || de.Kind != apperr.KindInvalid {
		t.Fatalf("expected bad_request on repeat, got %v", err)
	}
}

func TestSendCoachInvoice_guards(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	cfg := billing.Config{StripePriceCoach: "price_coach"}
	env := billingtest.NewEnv(t, pool, &billingtest.StubStripe{}, nil, nil, cfg)

	// Allocated (not yet invoiced) → coach folds into balance instead.
	allocated := billingtest.InsertCoachGroup(ctx, t, pool, domain.BillingAllocated, "lodge", false)
	if err := env.Invoicing.SendCoachInvoice(ctx, allocated, "Admin", domain.SkipVersionCheck); err == nil {
		t.Fatal("expected rejection for allocated group")
	}

	// Sponsored group → never coach-charged.
	free := billingtest.InsertCoachGroup(ctx, t, pool, domain.BillingInvoiced, "lodge", true)
	err := env.Invoicing.SendCoachInvoice(ctx, free, "Admin", domain.SkipVersionCheck)
	var de *apperr.Error
	if !errors.As(err, &de) || de.Kind != apperr.KindInvalid {
		t.Fatalf("expected bad_request for is_free, got %v", err)
	}

	// Invoiced group with no coach passengers → nothing to charge.
	var noCoach string
	if err := pool.QueryRow(ctx, `
		INSERT INTO registration_groups (
			contact_first_name, contact_last_name, contact_email, contact_phone,
			payment_status, total_amount_pence, currency, billing_status
		) VALUES ('No', 'Coach', 'nocoach@example.com', '07000000000', 'paid', 5000, 'GBP', 'invoiced')
		RETURNING id`).Scan(&noCoach); err != nil {
		t.Fatalf("insert no-coach group: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO registrations (
			group_id, is_main_contact, first_name, last_name, gender, age,
			cell_leader_name, is_cell_leader, attendance_type, needs_coach
		) VALUES ($1, true, 'No', 'Coach', 'male', 30, 'Leader', false, 'full_week', false)`,
		noCoach); err != nil {
		t.Fatalf("insert no-coach camper: %v", err)
	}
	if err := env.Invoicing.SendCoachInvoice(ctx, noCoach, "Admin", domain.SkipVersionCheck); err == nil {
		t.Fatal("expected rejection when no coach passengers")
	}
}

func TestWaiveCoachFee_notYetSent(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	stripe := &billingtest.StubStripe{
		VoidInvIdem: func(context.Context, string) error {
			t.Fatal("unexpected VoidInvoiceIdempotent")
			return nil
		},
	}
	cfg := billing.Config{StripePriceCoach: "price_coach"}
	env := billingtest.NewEnv(t, pool, stripe, nil, nil, cfg)
	repo := env.Repo

	groupID := billingtest.InsertCoachGroup(ctx, t, pool, domain.BillingAllocated, "lodge", false)
	if err := env.Invoicing.WaiveCoachFee(ctx, groupID, "Admin", domain.SkipVersionCheck); err != nil {
		t.Fatalf("WaiveCoachFee: %v", err)
	}
	g, _ := repo.FindGroupByID(ctx, groupID)
	if g == nil || g.CoachFeeWaivedAt == nil {
		t.Fatalf("coach_fee_waived_at not set: %+v", g)
	}
	if g.Version != 2 {
		t.Fatalf("version = %d, want 2", g.Version)
	}
	if g.LastAction == nil || *g.LastAction != "coach_fee_waived" {
		t.Fatalf("last_action = %v", g.LastAction)
	}
}

func TestWaiveCoachFee_voidOpenInvoice(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	var voidedID string
	stripe := &billingtest.StubStripe{
		EnsureCust: func(_ context.Context, _, _, _, _ string) (string, error) {
			return "cus_seed", nil
		},
		CreateInv: func(_ context.Context, _, _ string, _ []billing.InvoiceLine, _ int64, invoiceType string) (billing.InvoiceResult, error) {
			if invoiceType != "coach" {
				t.Fatalf("invoice type = %q, want coach", invoiceType)
			}
			return billing.InvoiceResult{ID: "in_coach_open", StripeEmailed: true}, nil
		},
		VoidInvIdem: func(_ context.Context, id string) error {
			voidedID = id
			return nil
		},
	}
	cfg := billing.Config{StripePriceCoach: "price_coach"}
	env := billingtest.NewEnv(t, pool, stripe, nil, nil, cfg)
	repo := env.Repo

	groupID := billingtest.InsertCoachGroup(ctx, t, pool, domain.BillingBalancePaid, "lodge", false)
	if err := env.Invoicing.SendCoachInvoice(ctx, groupID, "Admin", domain.SkipVersionCheck); err != nil {
		t.Fatalf("SendCoachInvoice: %v", err)
	}
	if err := env.Invoicing.WaiveCoachFee(ctx, groupID, "Admin", domain.SkipVersionCheck); err != nil {
		t.Fatalf("WaiveCoachFee: %v", err)
	}
	if voidedID != "in_coach_open" {
		t.Fatalf("voided invoice = %q, want in_coach_open", voidedID)
	}
	g, _ := repo.FindGroupByID(ctx, groupID)
	if g == nil || g.CoachFeeWaivedAt == nil {
		t.Fatalf("coach_fee_waived_at not set: %+v", g)
	}
	if g.StripeCoachInvoiceID != nil {
		t.Fatalf("stripe_coach_invoice_id should be cleared: %+v", g.StripeCoachInvoiceID)
	}
}

func TestWaiveCoachFee_guards(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	cfg := billing.Config{StripePriceCoach: "price_coach"}
	env := billingtest.NewEnv(t, pool, &billingtest.StubStripe{}, nil, nil, cfg)
	repo := env.Repo

	t.Run("folded_in_balance", func(t *testing.T) {
		if err := repo.UpdateAccommodationStripePrice(ctx, "lodge", "price_lodge"); err != nil {
			t.Fatalf("set lodge price: %v", err)
		}
		stub := &billingtest.StubStripe{
			EnsureCust: func(_ context.Context, _, _, _, _ string) (string, error) {
				return "cus_seed", nil
			},
			CreateInv: func(_ context.Context, _, _ string, _ []billing.InvoiceLine, _ int64, _ string) (billing.InvoiceResult, error) {
				return billing.InvoiceResult{ID: "in_balance", StripeEmailed: true}, nil
			},
		}
		cfg := billing.Config{StripePriceCoach: "price_coach"}
		env := billingtest.NewEnv(t, pool, stub, nil, nil, cfg)
		groupID := billingtest.InsertCoachGroup(ctx, t, pool, domain.BillingAllocated, "lodge", false)
		if err := env.Invoicing.SendInvoice(ctx, groupID, "Admin", domain.SkipVersionCheck); err != nil {
			t.Fatalf("SendInvoice: %v", err)
		}
		err := env.Invoicing.WaiveCoachFee(ctx, groupID, "Admin", domain.SkipVersionCheck)
		var de *apperr.Error
		if !errors.As(err, &de) || de.Kind != apperr.KindInvalid {
			t.Fatalf("expected bad_request, got %v", err)
		}
	})

	t.Run("already_paid", func(t *testing.T) {
		groupID := billingtest.InsertCoachGroup(ctx, t, pool, domain.BillingBalancePaid, "lodge", false)
		if err := invstorage.NewRepository(pool).SetCoachInvoiceMeta(ctx, groupID, "in_paid", time.Now().Add(15*24*time.Hour), domain.ActionMeta{
			Actor: "Admin", Action: "coach_invoice_sent", ExpectedVersion: domain.SkipVersionCheck,
		}); err != nil {
			t.Fatalf("seed coach invoice: %v", err)
		}
		if err := env.Reconcile.HandleCoachInvoicePaid(ctx, "in_paid", groupID); err != nil {
			t.Fatalf("HandleCoachInvoicePaid: %v", err)
		}
		err := env.Invoicing.WaiveCoachFee(ctx, groupID, "Admin", domain.SkipVersionCheck)
		var de *apperr.Error
		if !errors.As(err, &de) || de.Kind != apperr.KindInvalid {
			t.Fatalf("expected bad_request, got %v", err)
		}
	})
}

func TestSendCoachInvoice_waivedGroup(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	cfg := billing.Config{StripePriceCoach: "price_coach"}
	env := billingtest.NewEnv(t, pool, &billingtest.StubStripe{}, nil, nil, cfg)

	groupID := billingtest.InsertCoachGroup(ctx, t, pool, domain.BillingBalancePaid, "lodge", false)
	if err := env.Invoicing.WaiveCoachFee(ctx, groupID, "Admin", domain.SkipVersionCheck); err != nil {
		t.Fatalf("WaiveCoachFee: %v", err)
	}
	err := env.Invoicing.SendCoachInvoice(ctx, groupID, "Admin", domain.SkipVersionCheck)
	var de *apperr.Error
	if !errors.As(err, &de) || de.Kind != apperr.KindInvalid {
		t.Fatalf("expected bad_request, got %v", err)
	}
}

func TestSendInvoice_skipsCoachWhenWaived(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	cfg := billing.Config{StripePriceCoach: "price_coach"}
	var captured []billing.InvoiceLine
	stub := &billingtest.StubStripe{
		EnsureCust: func(_ context.Context, _, _, _, _ string) (string, error) {
			return "cus_seed", nil
		},
		CreateInv: func(_ context.Context, _, _ string, lines []billing.InvoiceLine, _ int64, invoiceType string) (billing.InvoiceResult, error) {
			captured = lines
			if invoiceType != "balance" {
				t.Fatalf("invoice type = %q, want balance", invoiceType)
			}
			return billing.InvoiceResult{ID: "in_balance", StripeEmailed: true}, nil
		},
	}
	env := billingtest.NewEnv(t, pool, stub, nil, nil, cfg)
	repo := env.Repo
	if err := repo.UpdateAccommodationStripePrice(ctx, "lodge", "price_lodge"); err != nil {
		t.Fatalf("set lodge price: %v", err)
	}

	groupID := billingtest.InsertCoachGroup(ctx, t, pool, domain.BillingAllocated, "lodge", false)
	if err := env.Invoicing.WaiveCoachFee(ctx, groupID, "Admin", domain.SkipVersionCheck); err != nil {
		t.Fatalf("WaiveCoachFee: %v", err)
	}
	if err := env.Invoicing.SendInvoice(ctx, groupID, "Admin", domain.SkipVersionCheck); err != nil {
		t.Fatalf("SendInvoice: %v", err)
	}
	for _, l := range captured {
		if l.PriceID == "price_coach" {
			t.Fatalf("coach line should be omitted when waived: %+v", captured)
		}
	}
	g, _ := repo.FindGroupByID(ctx, groupID)
	if g == nil || g.CoachIncludedInBalance {
		t.Fatalf("coach_included_in_balance should be false: %+v", g)
	}
}

func TestUnwaiveCoachFee(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	stub := &billingtest.StubStripe{
		EnsureCust: func(_ context.Context, _, _, _, _ string) (string, error) {
			return "cus_seed", nil
		},
		CreateInv: func(_ context.Context, _, _ string, _ []billing.InvoiceLine, _ int64, invoiceType string) (billing.InvoiceResult, error) {
			if invoiceType != "coach" {
				t.Fatalf("invoice type = %q, want coach", invoiceType)
			}
			return billing.InvoiceResult{ID: "in_coach_after", StripeEmailed: true}, nil
		},
	}
	cfg := billing.Config{StripePriceCoach: "price_coach"}
	env := billingtest.NewEnv(t, pool, stub, nil, nil, cfg)
	repo := env.Repo

	groupID := billingtest.InsertCoachGroup(ctx, t, pool, domain.BillingBalancePaid, "lodge", false)
	if err := env.Invoicing.WaiveCoachFee(ctx, groupID, "Admin", domain.SkipVersionCheck); err != nil {
		t.Fatalf("WaiveCoachFee: %v", err)
	}
	if err := env.Invoicing.UnwaiveCoachFee(ctx, groupID, "Admin", domain.SkipVersionCheck); err != nil {
		t.Fatalf("UnwaiveCoachFee: %v", err)
	}
	g, _ := repo.FindGroupByID(ctx, groupID)
	if g == nil || g.CoachFeeWaivedAt != nil {
		t.Fatalf("coach_fee_waived_at should be cleared: %+v", g)
	}
	if err := env.Invoicing.SendCoachInvoice(ctx, groupID, "Admin", domain.SkipVersionCheck); err != nil {
		t.Fatalf("SendCoachInvoice after unwaive: %v", err)
	}
	g, _ = repo.FindGroupByID(ctx, groupID)
	if g == nil || g.StripeCoachInvoiceID == nil || *g.StripeCoachInvoiceID != "in_coach_after" {
		t.Fatalf("coach invoice not sent after unwaive: %+v", g)
	}
}

func TestSendInvoice_rejectsPrepaidGroup(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	cfg := billing.Config{}
	env := billingtest.NewEnv(t, pool, &billingtest.StubStripe{}, nil, nil, cfg)

	var groupID string
	err := pool.QueryRow(ctx, `
		INSERT INTO registration_groups (
			contact_first_name, contact_last_name, contact_email, contact_phone,
			payment_status, total_amount_pence, currency, billing_status, paid_in_full_at_registration
		) VALUES ('Pre', 'Paid', 'prepaid2@example.com', '07000000000', 'paid', 70000, 'GBP', 'balance_paid', true)
		RETURNING id`).Scan(&groupID)
	if err != nil {
		t.Fatalf("insert group: %v", err)
	}

	err = env.Invoicing.SendInvoice(ctx, groupID, "Admin", domain.SkipVersionCheck)
	var de *apperr.Error
	if !errors.As(err, &de) || de.Kind != apperr.KindInvalid {
		t.Fatalf("expected bad_request, got %v", err)
	}
	if !strings.Contains(de.Message, "paid in full at registration") {
		t.Fatalf("unexpected message: %q", de.Message)
	}
}

func TestSendInvoicesBulk_skipsPrepaidGroup(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	cfg := billing.Config{}
	env := billingtest.NewEnv(t, pool, &billingtest.StubStripe{}, nil, nil, cfg)

	var groupID string
	err := pool.QueryRow(ctx, `
		INSERT INTO registration_groups (
			contact_first_name, contact_last_name, contact_email, contact_phone,
			payment_status, total_amount_pence, currency, billing_status, paid_in_full_at_registration
		) VALUES ('Pre', 'Paid', 'prepaid3@example.com', '07000000000', 'paid', 70000, 'GBP', 'balance_paid', true)
		RETURNING id`).Scan(&groupID)
	if err != nil {
		t.Fatalf("insert group: %v", err)
	}

	errs := env.Invoicing.SendInvoicesBulk(ctx, "Admin", []string{groupID})
	if len(errs) != 0 {
		t.Fatalf("expected prepaid group skipped silently, got errs=%v", errs)
	}
}

func TestMarkBalancePaidManually_allocatedGroupSkipsStripe(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	mailer := &billingtest.RecordingMailer{}
	voids := 0
	stub := &billingtest.StubStripe{
		VoidInvIdem: func(context.Context, string) error {
			voids++
			return nil
		},
	}
	cfg := billing.Config{}
	env := billingtest.NewEnv(t, pool, stub, mailer, nil, cfg)
	repo := env.Repo

	groupID := billingtest.InsertCoachGroup(ctx, t, pool, domain.BillingAllocated, "lodge", false)

	if err := env.Invoicing.MarkBalancePaidManually(ctx, groupID, "Diane", domain.SkipVersionCheck); err != nil {
		t.Fatalf("MarkBalancePaidManually: %v", err)
	}

	g, _ := repo.FindGroupByID(ctx, groupID)
	if g.BillingStatus != domain.BillingBalancePaid {
		t.Fatalf("billing_status = %q, want balance_paid", g.BillingStatus)
	}
	if g.BalancePaidAt == nil {
		t.Fatal("balance_paid_at not set")
	}
	if voids != 0 {
		t.Fatalf("void called %d times, want 0 for an uninvoiced group", voids)
	}
	if len(mailer.BalancePaid) != 1 {
		t.Fatalf("confirmation emails = %d, want 1", len(mailer.BalancePaid))
	}
	if mailer.BalancePaid[0].AmountLabel != "" {
		t.Fatalf("amount label = %q, want blank when there is no invoice", mailer.BalancePaid[0].AmountLabel)
	}

	// The family keeps the beds they were given; marking paid is not a release.
	campers, _ := repo.CampersForGroup(ctx, groupID)
	for _, c := range campers {
		if c.AllocatedAccommodationCode == nil || *c.AllocatedAccommodationCode != "lodge" {
			t.Fatalf("camper %s lost their allocation", c.FirstName)
		}
	}
}

func TestMarkBalancePaidManually_invoicedVoidsInvoice(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	mailer := &billingtest.RecordingMailer{}

	var voided []string
	stub := &billingtest.StubStripe{
		GetInvoiceFn: func(_ context.Context, id string) (billing.InvoiceResult, error) {
			return billing.InvoiceResult{ID: id, AmountDuePence: 25000, Currency: "gbp"}, nil
		},
		VoidInvIdem: func(_ context.Context, id string) error {
			voided = append(voided, id)
			return nil
		},
	}
	cfg := billing.Config{}
	env := billingtest.NewEnv(t, pool, stub, mailer, nil, cfg)
	repo := env.Repo

	groupID := billingtest.InsertCoachGroup(ctx, t, pool, domain.BillingInvoiced, "lodge", false)
	if _, err := pool.Exec(ctx, `
		UPDATE registration_groups SET stripe_invoice_id = 'in_bank_transfer' WHERE id = $1`, groupID); err != nil {
		t.Fatalf("seed invoice id: %v", err)
	}

	if err := env.Invoicing.MarkBalancePaidManually(ctx, groupID, "Diane", domain.SkipVersionCheck); err != nil {
		t.Fatalf("MarkBalancePaidManually: %v", err)
	}

	if len(voided) != 1 || voided[0] != "in_bank_transfer" {
		t.Fatalf("voided = %v, want the open invoice voided once", voided)
	}
	g, _ := repo.FindGroupByID(ctx, groupID)
	if g.BillingStatus != domain.BillingBalancePaid {
		t.Fatalf("billing_status = %q, want balance_paid", g.BillingStatus)
	}
	if len(mailer.BalancePaid) != 1 || mailer.BalancePaid[0].AmountLabel != "£250.00" {
		t.Fatalf("confirmation = %+v, want one email showing £250.00", mailer.BalancePaid)
	}
}

func TestMarkBalancePaidManually_voidFailureLeavesGroupUnpaid(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	mailer := &billingtest.RecordingMailer{}
	stub := &billingtest.StubStripe{
		GetInvoiceFn: func(_ context.Context, id string) (billing.InvoiceResult, error) {
			return billing.InvoiceResult{ID: id, AmountDuePence: 25000, Currency: "gbp"}, nil
		},
		VoidInvIdem: func(context.Context, string) error {
			return errors.New("stripe down")
		},
	}
	cfg := billing.Config{}
	env := billingtest.NewEnv(t, pool, stub, mailer, nil, cfg)
	repo := env.Repo

	groupID := billingtest.InsertCoachGroup(ctx, t, pool, domain.BillingInvoiced, "lodge", false)
	if _, err := pool.Exec(ctx, `
		UPDATE registration_groups SET stripe_invoice_id = 'in_void_fails' WHERE id = $1`, groupID); err != nil {
		t.Fatalf("seed invoice id: %v", err)
	}

	err := env.Invoicing.MarkBalancePaidManually(ctx, groupID, "Diane", domain.SkipVersionCheck)
	var de *apperr.Error
	if !errors.As(err, &de) || de.Kind != apperr.KindInvalid {
		t.Fatalf("expected bad_request when the void fails, got %v", err)
	}

	// A live payment link plus a paid-looking group is the double charge we are
	// preventing, so nothing may have changed.
	g, _ := repo.FindGroupByID(ctx, groupID)
	if g.BillingStatus != domain.BillingInvoiced {
		t.Fatalf("billing_status = %q, want it left invoiced", g.BillingStatus)
	}
	if g.BalancePaidAt != nil {
		t.Fatal("balance_paid_at set despite the void failing")
	}
	if len(mailer.BalancePaid) != 0 {
		t.Fatalf("sent %d confirmations for a payment that was not recorded", len(mailer.BalancePaid))
	}
}

func TestMarkBalancePaidManually_guards(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	cfg := billing.Config{}
	env := billingtest.NewEnv(t, pool, &billingtest.StubStripe{}, nil, nil, cfg)

	expectBadRequest := func(t *testing.T, err error) {
		t.Helper()
		var de *apperr.Error
		if !errors.As(err, &de) || de.Kind != apperr.KindInvalid {
			t.Fatalf("expected bad_request, got %v", err)
		}
	}

	t.Run("already_balance_paid", func(t *testing.T) {
		groupID := billingtest.InsertCoachGroup(ctx, t, pool, domain.BillingBalancePaid, "lodge", false)
		expectBadRequest(t, env.Invoicing.MarkBalancePaidManually(ctx, groupID, "Diane", domain.SkipVersionCheck))
	})

	t.Run("church_sponsored", func(t *testing.T) {
		groupID := billingtest.InsertCoachGroup(ctx, t, pool, domain.BillingAllocated, "lodge", true)
		expectBadRequest(t, env.Invoicing.MarkBalancePaidManually(ctx, groupID, "Diane", domain.SkipVersionCheck))
	})

	t.Run("released", func(t *testing.T) {
		groupID := billingtest.InsertCoachGroup(ctx, t, pool, domain.BillingReleased, "lodge", false)
		expectBadRequest(t, env.Invoicing.MarkBalancePaidManually(ctx, groupID, "Diane", domain.SkipVersionCheck))
	})

	t.Run("deposit_unpaid", func(t *testing.T) {
		var groupID string
		if err := pool.QueryRow(ctx, `
			INSERT INTO registration_groups (
				contact_first_name, contact_last_name, contact_email, contact_phone,
				payment_status, total_amount_pence, currency, billing_status
			) VALUES ('No', 'Deposit', 'nodeposit@example.com', '07000000000', 'pending', 5000, 'GBP', 'allocated')
			RETURNING id`).Scan(&groupID); err != nil {
			t.Fatalf("insert group: %v", err)
		}
		expectBadRequest(t, env.Invoicing.MarkBalancePaidManually(ctx, groupID, "Diane", domain.SkipVersionCheck))
	})
}

func TestMarkBalancePaidManually_versionConflict(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	cfg := billing.Config{}
	env := billingtest.NewEnv(t, pool, &billingtest.StubStripe{}, nil, nil, cfg)
	repo := env.Repo

	groupID := billingtest.InsertCoachGroup(ctx, t, pool, domain.BillingAllocated, "lodge", false)

	err := env.Invoicing.MarkBalancePaidManually(ctx, groupID, "Diane", 99)
	var de *apperr.Error
	if !errors.As(err, &de) || de.Kind != apperr.KindConflict || de.Code != "stale_state" {
		t.Fatalf("expected stale_state conflict, got %v", err)
	}
	g, _ := repo.FindGroupByID(ctx, groupID)
	if g.BillingStatus != domain.BillingAllocated {
		t.Fatalf("billing_status = %q, want unchanged allocated", g.BillingStatus)
	}
}

func TestSendInvoice_billsOutstandingDeposit(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	cfg := billing.Config{}
	var captured []billing.InvoiceLine
	stub := &billingtest.StubStripe{
		EnsureCust: func(_ context.Context, _, _, _, _ string) (string, error) {
			return "cus_edit", nil
		},
		CreateInv: func(_ context.Context, _, _ string, lines []billing.InvoiceLine, _ int64, _ string) (billing.InvoiceResult, error) {
			captured = lines
			return billing.InvoiceResult{ID: "in_with_deposit", StripeEmailed: true}, nil
		},
	}
	env := billingtest.NewEnv(t, pool, stub, &billingtest.RecordingMailer{}, nil, cfg)
	repo := env.Repo
	if err := repo.UpdateAccommodationStripePrice(ctx, "lodge", "price_lodge"); err != nil {
		t.Fatalf("set lodge price: %v", err)
	}

	groupID, _, otherID := billingtest.InsertEditableGroup(ctx, t, pool, domain.BillingAllocated)
	if _, err := pool.Exec(ctx,
		`UPDATE registrations SET deposit_owed_pence = 5000 WHERE id = $1`, otherID); err != nil {
		t.Fatalf("seed deposit owed: %v", err)
	}

	if err := env.Invoicing.SendInvoice(ctx, groupID, "Diane", domain.SkipVersionCheck); err != nil {
		t.Fatalf("SendInvoice: %v", err)
	}

	var deposits []billing.InvoiceLine
	for _, l := range captured {
		if l.PriceID == "" {
			deposits = append(deposits, l)
		}
	}
	if len(deposits) != 1 {
		t.Fatalf("ad-hoc lines = %+v, want exactly one deposit line", captured)
	}
	if deposits[0].AmountPence != 5000 {
		t.Fatalf("deposit line amount = %d, want 5000", deposits[0].AmountPence)
	}
	if !strings.Contains(deposits[0].Description, "Priscilla Fundi") {
		t.Fatalf("deposit line description = %q, should name who it is for",
			deposits[0].Description)
	}
	if deposits[0].Currency != "GBP" {
		t.Fatalf("deposit line currency = %q", deposits[0].Currency)
	}
}

func TestMarkBalancePaid_settlesOutstandingDeposits(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	cfg := billing.Config{}
	env := billingtest.NewEnv(t, pool, &billingtest.StubStripe{}, &billingtest.RecordingMailer{}, nil, cfg)
	settle := map[string]func(string) error{
		"bank transfer": func(groupID string) error {
			return env.Invoicing.MarkBalancePaidManually(ctx, groupID, "Diane", domain.SkipVersionCheck)
		},
		"stripe webhook": func(groupID string) error {
			return env.Reconcile.HandleInvoicePaid(ctx, "in_settle", groupID, 17000, "gbp")
		},
	}

	for name, run := range settle {
		t.Run(name, func(t *testing.T) {
			cfg := billing.Config{}
			env := billingtest.NewEnv(t, pool, &billingtest.StubStripe{}, &billingtest.RecordingMailer{}, nil, cfg)
			repo := env.Repo
			groupID, _, otherID := billingtest.InsertEditableGroup(ctx, t, pool, domain.BillingAllocated)
			if _, err := pool.Exec(ctx,
				`UPDATE registrations SET deposit_owed_pence = 5000 WHERE id = $1`, otherID); err != nil {
				t.Fatalf("seed deposit owed: %v", err)
			}

			if err := run(groupID); err != nil {
				t.Fatalf("settle balance: %v", err)
			}

			campers, _ := repo.CampersForGroup(ctx, groupID)
			got := camperByID(t, campers, otherID)
			if got.DepositOwedPence != 0 {
				t.Fatalf("deposit_owed_pence = %d, want 0 once the balance is settled",
					got.DepositOwedPence)
			}
			if got.DepositPaidPence != 5000 {
				t.Fatalf("deposit_paid_pence = %d, want the 5000 they just paid",
					got.DepositPaidPence)
			}
		})
	}
}

func TestDepositOwed_clearedWithoutPaymentIsNotRecordedAsPaid(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	needsCatering := false
	cfg := billing.Config{}
	env := billingtest.NewEnvWithPrices(t, pool, &billingtest.StubStripe{}, &billingtest.RecordingMailer{}, nil, cfg, fakePrices{depositPence: 5000})

	clear := map[string]func(string, string) error{
		"waived": func(groupID, camperID string) error {
			_, err := env.Campers.WaiveCamperDeposit(ctx, groupID, camperID, "Diane", domain.SkipVersionCheck)
			return err
		},
		"converted to day visitor": func(groupID, camperID string) error {
			_, err := env.Campers.ConvertCamperToDayVisitor(
				ctx, groupID, camperID, "Diane", domain.SkipVersionCheck,
				camperedit.ConvertToDayVisitorRequest{
					Days:          []string{"mon"},
					TshirtOption:  domain.TshirtOptionNone,
					NeedsCatering: &needsCatering,
				})
			return err
		},
	}

	for name, run := range clear {
		t.Run(name, func(t *testing.T) {
			stub := &billingtest.StubStripe{
				EnsureCust: func(_ context.Context, _, _, _, _ string) (string, error) {
					return "cus_edit", nil
				},
			}
			cfg := billing.Config{
				DepositPricePence:  5000,
				StripePriceDayPass: "price_day",
			}
			env := billingtest.NewEnv(t, pool, stub, &billingtest.RecordingMailer{}, nil, cfg)
			repo := env.Repo
			groupID, _, otherID := billingtest.InsertEditableGroup(ctx, t, pool, domain.BillingAllocated)
			if _, err := pool.Exec(ctx,
				`UPDATE registrations SET deposit_owed_pence = 5000 WHERE id = $1`, otherID); err != nil {
				t.Fatalf("seed deposit owed: %v", err)
			}

			if err := run(groupID, otherID); err != nil {
				t.Fatalf("clear deposit: %v", err)
			}

			campers, _ := repo.CampersForGroup(ctx, groupID)
			got := camperByID(t, campers, otherID)
			if got.DepositOwedPence != 0 {
				t.Fatalf("deposit_owed_pence = %d, want 0", got.DepositOwedPence)
			}
			if got.DepositPaidPence != 0 {
				t.Fatalf("deposit_paid_pence = %d, want 0: nobody paid this deposit",
					got.DepositPaidPence)
			}

			// The point of all this: deleting afterwards keeps only the deposit
			// that came in at checkout, never the one nobody paid.
			sum, err := env.Invoicing.DeleteRegistration(ctx, groupID, "Diane", domain.SkipVersionCheck)
			if err != nil {
				t.Fatalf("DeleteRegistration: %v", err)
			}
			if sum.RetainedPence != 10000 {
				t.Fatalf("retained = %d, want only the 10000 paid at checkout",
					sum.RetainedPence)
			}
		})
	}
}
