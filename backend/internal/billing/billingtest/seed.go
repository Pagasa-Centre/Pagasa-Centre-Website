package billingtest

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"pagasacentre/backend/internal/registration/domain"
)

// InsertEditableGroup seeds a paid group with two full-week lodge campers for edit/remove tests.
func InsertEditableGroup(
	ctx context.Context,
	t *testing.T,
	pool *pgxpool.Pool,
	billingStatus domain.BillingStatus,
) (groupID, mainID, otherID string) {
	t.Helper()
	err := pool.QueryRow(ctx, `
		INSERT INTO registration_groups (
			contact_first_name, contact_last_name, contact_email, contact_phone,
			payment_status, stripe_customer_id, total_amount_pence, currency,
			billing_status, version
		) VALUES ('Grace', 'Fundi', 'grace@example.com', '07000000123', 'paid',
			'cus_edit', 10000, 'GBP', $1, 1)
		RETURNING id`, billingStatus).Scan(&groupID)
	if err != nil {
		t.Fatalf("insert group: %v", err)
	}
	err = pool.QueryRow(ctx, `
		INSERT INTO registrations (
			group_id, is_main_contact, first_name, last_name, gender, age,
			cell_leader_name, is_cell_leader, attendance_type, needs_coach,
			shirt_size, accommodation_first_choice, accommodation_second_choice,
			allocated_accommodation_code
		) VALUES ($1, true, 'Grace', 'Fundi', 'female', 34, 'Leader', false,
			'full_week', false, 'adult_m', 'lodge', 'static_caravan', 'lodge')
		RETURNING id`, groupID).Scan(&mainID)
	if err != nil {
		t.Fatalf("insert main camper: %v", err)
	}
	err = pool.QueryRow(ctx, `
		INSERT INTO registrations (
			group_id, is_main_contact, first_name, last_name, gender, age,
			cell_leader_name, is_cell_leader, attendance_type, needs_coach,
			shirt_size, accommodation_first_choice, accommodation_second_choice,
			allocated_accommodation_code
		) VALUES ($1, false, 'Priscilla', 'Fundi', 'female', 21, 'Leader', false,
			'full_week', false, 'adult_s', 'lodge', 'static_caravan', 'lodge')
		RETURNING id`, groupID).Scan(&otherID)
	if err != nil {
		t.Fatalf("insert second camper: %v", err)
	}
	return groupID, mainID, otherID
}

// InsertThreeCamperGroup seeds Sam Family with three allocated full-week campers.
func InsertThreeCamperGroup(
	ctx context.Context,
	t *testing.T,
	pool *pgxpool.Pool,
	billingStatus domain.BillingStatus,
	stripeInvoiceID *string,
) (groupID, mainID, camper2ID, camper3ID string) {
	t.Helper()
	inv := any(nil)
	if stripeInvoiceID != nil {
		inv = *stripeInvoiceID
	}
	err := pool.QueryRow(ctx, `
		INSERT INTO registration_groups (
			contact_first_name, contact_last_name, contact_email, contact_phone,
			payment_status, stripe_payment_intent_id, stripe_invoice_id,
			total_amount_pence, currency, billing_status, version
		) VALUES ('Sam', 'Family', 'family@example.com', '07000000000', 'paid', 'pi_test', $1, 15000, 'GBP', $2, 1)
		RETURNING id`, inv, billingStatus).Scan(&groupID)
	if err != nil {
		t.Fatalf("insert group: %v", err)
	}
	for _, spec := range []struct {
		id    *string
		main  bool
		name  string
		alloc *string
	}{
		{&mainID, true, "Sam", StrPtr("lodge")},
		{&camper2ID, false, "Alex", StrPtr("lodge")},
		{&camper3ID, false, "Jordan", StrPtr("tent")},
	} {
		err = pool.QueryRow(ctx, `
			INSERT INTO registrations (
				group_id, is_main_contact, first_name, last_name, gender, age,
				cell_leader_name, is_cell_leader, attendance_type,
				allocated_accommodation_code
			) VALUES ($1, $2, $3, 'Test', 'male', 25, 'Leader', false, 'full_week', $4)
			RETURNING id`, groupID, spec.main, spec.name, spec.alloc).Scan(spec.id)
		if err != nil {
			t.Fatalf("insert camper %s: %v", spec.name, err)
		}
	}
	return groupID, mainID, camper2ID, camper3ID
}

// InsertCoachGroup seeds one full-week camper who needs coach transport.
func InsertCoachGroup(
	ctx context.Context,
	t *testing.T,
	pool *pgxpool.Pool,
	billingStatus domain.BillingStatus,
	tier string,
	isFree bool,
) string {
	t.Helper()
	var groupID string
	err := pool.QueryRow(ctx, `
		INSERT INTO registration_groups (
			contact_first_name, contact_last_name, contact_email, contact_phone,
			payment_status, stripe_customer_id, total_amount_pence, currency, billing_status, is_free, version
		) VALUES ('Coach', 'Rider', 'coach@example.com', '07000000000', 'paid', 'cus_seed', 5000, 'GBP', $1, $2, 1)
		RETURNING id`, billingStatus, isFree).Scan(&groupID)
	if err != nil {
		t.Fatalf("insert coach group: %v", err)
	}
	_, err = pool.Exec(ctx, `
		INSERT INTO registrations (
			group_id, is_main_contact, first_name, last_name, gender, age,
			cell_leader_name, is_cell_leader, attendance_type, needs_coach,
			allocated_accommodation_code
		) VALUES ($1, true, 'Coach', 'Rider', 'male', 30, 'Leader', false, 'full_week', true, $2)`,
		groupID, tier)
	if err != nil {
		t.Fatalf("insert coach camper: %v", err)
	}
	return groupID
}

// InsertTwoCoachGroup seeds two coach-eligible campers on one group.
func InsertTwoCoachGroup(
	ctx context.Context,
	t *testing.T,
	pool *pgxpool.Pool,
	billingStatus domain.BillingStatus,
) (groupID, camper1ID, camper2ID string) {
	t.Helper()
	err := pool.QueryRow(ctx, `
		INSERT INTO registration_groups (
			contact_first_name, contact_last_name, contact_email, contact_phone,
			payment_status, stripe_customer_id, total_amount_pence, currency, billing_status, version
		) VALUES ('Aliyah', 'Oliveros', 'aliyah@example.com', '07000000001', 'paid', 'cus_two', 10000, 'GBP', $1, 1)
		RETURNING id`, billingStatus).Scan(&groupID)
	if err != nil {
		t.Fatalf("insert two-coach group: %v", err)
	}
	err = pool.QueryRow(ctx, `
		INSERT INTO registrations (
			group_id, is_main_contact, first_name, last_name, gender, age,
			cell_leader_name, is_cell_leader, attendance_type, needs_coach,
			allocated_accommodation_code
		) VALUES ($1, true, 'Aliyah', 'Oliveros', 'female', 25, 'Leader', false, 'full_week', true, 'lodge')
		RETURNING id`, groupID).Scan(&camper1ID)
	if err != nil {
		t.Fatalf("insert camper 1: %v", err)
	}
	err = pool.QueryRow(ctx, `
		INSERT INTO registrations (
			group_id, is_main_contact, first_name, last_name, gender, age,
			cell_leader_name, is_cell_leader, attendance_type, needs_coach,
			allocated_accommodation_code
		) VALUES ($1, false, 'Stephanie', 'Alves', 'female', 19, 'Leader', false, 'full_week', true, 'lodge')
		RETURNING id`, groupID).Scan(&camper2ID)
	if err != nil {
		t.Fatalf("insert camper 2: %v", err)
	}
	return groupID, camper1ID, camper2ID
}

// InsertDayPassGroup seeds one day-pass camper.
func InsertDayPassGroup(
	ctx context.Context,
	t *testing.T,
	pool *pgxpool.Pool,
	billingStatus domain.BillingStatus,
) (groupID, camperID string) {
	t.Helper()
	err := pool.QueryRow(ctx, `
		INSERT INTO registration_groups (
			contact_first_name, contact_last_name, contact_email, contact_phone,
			payment_status, total_amount_pence, currency, billing_status, version
		) VALUES ('Day', 'Pass', 'daypass@example.com', '07000000000', 'paid', 0, 'GBP', $1, 1)
		RETURNING id`, billingStatus).Scan(&groupID)
	if err != nil {
		t.Fatalf("insert group: %v", err)
	}
	tshirt := domain.TshirtOptionTeamActivities
	catering := true
	shirt := "adult_m"
	dietary := "None"
	err = pool.QueryRow(ctx, `
		INSERT INTO registrations (
			group_id, is_main_contact, first_name, last_name, gender, age,
			cell_leader_name, is_cell_leader, attendance_type,
			day_pass_days, day_pass_tshirt_option, day_pass_needs_catering,
			shirt_size, dietary_requirements
		) VALUES ($1, true, 'Day', 'Passer', 'female', 30, 'Leader', false, 'day_pass',
			'{mon,tue}', $2, $3, $4, $5)
		RETURNING id`, groupID, tshirt, catering, shirt, dietary).Scan(&camperID)
	if err != nil {
		t.Fatalf("insert camper: %v", err)
	}
	return groupID, camperID
}
