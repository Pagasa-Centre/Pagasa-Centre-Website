package storage

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"pagasacentre/backend/internal/registration/domain"
	"pagasacentre/backend/internal/registration/storage/groupsql"
	commondb "pagasacentre/backend/pkg/commonlibrary/db"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// UpdateCamperDetailsMeta rewrites one camper's details. tr, when it changes
// billing_status, clears invoice fields after a now-stale invoice was voided.
func (r *Repository) UpdateCamperDetailsMeta(
	ctx context.Context,
	groupID, camperID string,
	d domain.CamperDetails,
	meta domain.ActionMeta,
	tr domain.BillingTransition,
) error {
	tx, err := commondb.Begin(ctx, r.pool)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	g, err := groupsql.LockGroup(ctx, tx, groupID)
	if err != nil {
		return err
	}
	if g == nil {
		return fmt.Errorf("group %q not found", groupID)
	}
	if err := groupsql.CheckVersion(g, meta); err != nil {
		return err
	}

	tag, err := tx.Exec(ctx, `
		UPDATE registrations SET
			first_name = $3,
			last_name = $4,
			gender = $5,
			age = $6,
			cell_leader_name = $7,
			is_cell_leader = $8,
			shirt_size = CASE WHEN attendance_type = 'full_week'
				THEN $9 ELSE shirt_size END,
			dietary_requirements = CASE WHEN attendance_type = 'full_week'
				THEN $10 ELSE dietary_requirements END,
			accommodation_first_choice = CASE WHEN attendance_type = 'full_week'
				THEN $11 ELSE accommodation_first_choice END,
			accommodation_second_choice = CASE WHEN attendance_type = 'full_week'
				THEN $12 ELSE accommodation_second_choice END,
			allocated_accommodation_code = CASE WHEN attendance_type = 'full_week'
				THEN $13 ELSE allocated_accommodation_code END,
			allocated_unit_code = CASE WHEN attendance_type = 'full_week'
				THEN $14 ELSE allocated_unit_code END,
			billed_stripe_price_id = CASE WHEN attendance_type = 'full_week'
				THEN $15 ELSE billed_stripe_price_id END
		WHERE id = $1 AND group_id = $2`,
		camperID, groupID,
		d.FirstName, d.LastName, d.Gender, d.Age, d.CellLeaderName, d.IsCellLeader,
		d.ShirtSize, d.DietaryRequirements,
		d.AccommodationFirstChoice, d.AccommodationSecondChoice,
		d.AllocatedAccommodationCode, d.AllocatedUnitCode, d.BilledStripePriceID,
	)
	if err != nil {
		return fmt.Errorf("update camper details: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("camper %q not in group %q", camperID, groupID)
	}

	extra := ""
	if tr.ChangesBilling() {
		extra = `, stripe_invoice_id = NULL, invoice_due_at = NULL`
	}
	if err := groupsql.Stamp(ctx, tx, groupID, meta, tr, extra); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// AddCamperMeta inserts a camper into an existing group and returns the new row's
// id. depositOwedPence records a deposit this person still owes because the
// group's deposit checkout was already settled before they joined.
// tr carries any billing fallout from adding someone to the roster.
func (r *Repository) AddCamperMeta(
	ctx context.Context,
	groupID string,
	c domain.NewCamper,
	depositOwedPence int,
	meta domain.ActionMeta,
	tr domain.BillingTransition,
) (string, error) {
	tx, err := commondb.Begin(ctx, r.pool)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)

	g, err := groupsql.LockGroup(ctx, tx, groupID)
	if err != nil {
		return "", err
	}
	if g == nil {
		return "", fmt.Errorf("group %q not found", groupID)
	}
	if err := groupsql.CheckVersion(g, meta); err != nil {
		return "", err
	}

	var (
		shirtSize, dietary, firstChoice, secondChoice, roommate, tshirtOption *string
		needsCoach, needsCatering                                             *bool
		days                                                                  []string
	)
	switch c.Attendance.Type {
	case domain.AttendanceFullWeek:
		shirtSize = strPtr(c.Attendance.ShirtSize)
		dietary = strPtr(c.Attendance.DietaryRequirements)
		needsCoach = c.Attendance.NeedsCoach
		firstChoice = strPtr(c.Attendance.AccommodationFirstChoice)
		secondChoice = strPtr(c.Attendance.AccommodationSecondChoice)
		roommate = strPtr(c.Attendance.RoommateRequests)
	case domain.AttendanceDayPass:
		dietary = strPtr(c.Attendance.DietaryRequirements)
		if s := c.Attendance.ShirtSize; s != "" {
			shirtSize = &s
		}
		days = c.Attendance.Days
		o := c.Attendance.TshirtOption
		tshirtOption = &o
		needsCatering = c.Attendance.NeedsCatering
	}

	// is_main_contact is deliberately false: a group has exactly one, and it is
	// moved with SetMainContactMeta rather than created alongside an arrival.
	var camperID string
	err = tx.QueryRow(ctx, `
		INSERT INTO registrations (
			group_id, is_main_contact, first_name, last_name, gender, age,
			cell_leader_name, is_cell_leader, attendance_type,
			shirt_size, dietary_requirements, needs_coach,
			accommodation_first_choice, accommodation_second_choice, roommate_requests,
			day_pass_days, day_pass_tshirt_option, day_pass_needs_catering,
			deposit_owed_pence
		) VALUES (
			$1,false,$2,$3,$4,$5,
			$6,$7,$8,
			$9,$10,$11,
			$12,$13,$14,
			$15,$16,$17,
			$18
		) RETURNING id`,
		groupID, c.FirstName, c.LastName, c.Gender, c.Age,
		c.CellLeaderName, c.IsCellLeader, c.Attendance.Type,
		shirtSize, dietary, needsCoach,
		firstChoice, secondChoice, roommate,
		days, tshirtOption, needsCatering,
		depositOwedPence,
	).Scan(&camperID)
	if err != nil {
		return "", fmt.Errorf("add camper: %w", err)
	}

	extra := ""
	if tr.ChangesBilling() {
		extra = `, stripe_invoice_id = NULL, invoice_due_at = NULL`
	}
	if err := groupsql.Stamp(ctx, tx, groupID, meta, tr, extra); err != nil {
		return "", err
	}
	if err := tx.Commit(ctx); err != nil {
		return "", err
	}
	return camperID, nil
}

// SetMainContactMeta moves the main-contact marker to one camper. Clearing every
// other row in the same statement keeps the "exactly one main contact per group"
// invariant true at all times, since the two writes share a transaction.
func (r *Repository) SetMainContactMeta(
	ctx context.Context,
	groupID, camperID string,
	meta domain.ActionMeta,
) error {
	tx, err := commondb.Begin(ctx, r.pool)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	g, err := groupsql.LockGroup(ctx, tx, groupID)
	if err != nil {
		return err
	}
	if g == nil {
		return fmt.Errorf("group %q not found", groupID)
	}
	if err := groupsql.CheckVersion(g, meta); err != nil {
		return err
	}

	tag, err := tx.Exec(ctx, `
		UPDATE registrations
		   SET is_main_contact = (id = $2)
		 WHERE group_id = $1`,
		groupID, camperID)
	if err != nil {
		return fmt.Errorf("set main contact: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("group %q has no campers", groupID)
	}
	if err := groupsql.Stamp(ctx, tx, groupID, meta, domain.BillingTransition{}, ""); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// WaiveCamperDepositMeta clears one camper's outstanding deposit so it is never
// billed.
func (r *Repository) WaiveCamperDepositMeta(
	ctx context.Context,
	groupID, camperID string,
	meta domain.ActionMeta,
) error {
	tx, err := commondb.Begin(ctx, r.pool)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	g, err := groupsql.LockGroup(ctx, tx, groupID)
	if err != nil {
		return err
	}
	if g == nil {
		return fmt.Errorf("group %q not found", groupID)
	}
	if err := groupsql.CheckVersion(g, meta); err != nil {
		return err
	}

	tag, err := tx.Exec(ctx,
		`UPDATE registrations SET deposit_owed_pence = 0 WHERE id = $1 AND group_id = $2`,
		camperID, groupID)
	if err != nil {
		return fmt.Errorf("waive camper deposit: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("camper %q not in group %q", camperID, groupID)
	}
	if err := groupsql.Stamp(ctx, tx, groupID, meta, domain.BillingTransition{}, ""); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *Repository) DeleteCamperMeta(ctx context.Context, groupID, camperID string, meta domain.ActionMeta, tr domain.BillingTransition) error {
	tx, err := commondb.Begin(ctx, r.pool)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	g, err := groupsql.LockGroup(ctx, tx, groupID)
	if err != nil {
		return err
	}
	if g == nil {
		return fmt.Errorf("group %q not found", groupID)
	}
	if err := groupsql.CheckVersion(g, meta); err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, `DELETE FROM registrations WHERE id = $1 AND group_id = $2`, camperID, groupID)
	if err != nil {
		return fmt.Errorf("delete camper: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("camper %q not in group %q", camperID, groupID)
	}
	extra := ""
	if tr.ChangesBilling() {
		extra = `, stripe_invoice_id = NULL, invoice_due_at = NULL`
	}
	if err := groupsql.Stamp(ctx, tx, groupID, meta, tr, extra); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// ConvertCamperToDayPassMeta rewrites one camper as a day-visitor, clearing
// accommodation/coach/allocation and recording any deposit credit applied in Stripe.

func (r *Repository) ConvertCamperToDayPassMeta(
	ctx context.Context,
	groupID, camperID string,
	dp domain.DayPassFields,
	depositCreditPence int,
	meta domain.ActionMeta,
	tr domain.BillingTransition,
) error {
	tx, err := commondb.Begin(ctx, r.pool)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	g, err := groupsql.LockGroup(ctx, tx, groupID)
	if err != nil {
		return err
	}
	if g == nil {
		return fmt.Errorf("group %q not found", groupID)
	}
	if err := groupsql.CheckVersion(g, meta); err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, `
		UPDATE registrations SET
			attendance_type = 'day_pass',
			day_pass_days = $3,
			day_pass_tshirt_option = $4,
			day_pass_needs_catering = $5,
			shirt_size = $6,
			dietary_requirements = COALESCE($7, dietary_requirements),
			needs_coach = NULL,
			accommodation_first_choice = NULL,
			accommodation_second_choice = NULL,
			roommate_requests = NULL,
			allocated_accommodation_code = NULL,
			allocated_unit_code = NULL,
			billed_stripe_price_id = NULL,
			deposit_credit_pence = $8,
			deposit_owed_pence = 0
		WHERE id = $1 AND group_id = $2`,
		camperID, groupID, dp.Days, dp.TshirtOption, dp.NeedsCatering, dp.ShirtSize, dp.Dietary, depositCreditPence)
	if err != nil {
		return fmt.Errorf("convert camper to day pass: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("camper %q not in group %q", camperID, groupID)
	}
	extra := ""
	if tr.ChangesBilling() {
		extra = `, stripe_invoice_id = NULL, invoice_due_at = NULL`
	}
	if err := groupsql.Stamp(ctx, tx, groupID, meta, tr, extra); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// UpdateDayPassCamperMeta updates day-pass shirt/catering/dietary fields without
// touching attendance, allocation, or billing status.

func (r *Repository) UpdateDayPassCamperMeta(
	ctx context.Context,
	groupID, camperID string,
	tshirtOption string,
	needsCatering bool,
	shirtSize, dietary *string,
	meta domain.ActionMeta,
) error {
	tx, err := commondb.Begin(ctx, r.pool)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	g, err := groupsql.LockGroup(ctx, tx, groupID)
	if err != nil {
		return err
	}
	if g == nil {
		return fmt.Errorf("group %q not found", groupID)
	}
	if err := groupsql.CheckVersion(g, meta); err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, `
		UPDATE registrations SET
			day_pass_tshirt_option = $3,
			day_pass_needs_catering = $4,
			shirt_size = $5,
			dietary_requirements = $6
		WHERE id = $1 AND group_id = $2`,
		camperID, groupID, tshirtOption, needsCatering, shirtSize, dietary)
	if err != nil {
		return fmt.Errorf("update day-pass camper: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("camper %q not in group %q", camperID, groupID)
	}
	if err := groupsql.Stamp(ctx, tx, groupID, meta, domain.BillingTransition{}, ""); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// UpdateCamperCoachMeta sets needs_coach for one full-week camper and optionally
// clears an open separate coach invoice on the group.

func (r *Repository) UpdateCamperCoachMeta(
	ctx context.Context,
	groupID, camperID string,
	needsCoach bool,
	clearCoachInvoice bool,
	meta domain.ActionMeta,
) error {
	tx, err := commondb.Begin(ctx, r.pool)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	g, err := groupsql.LockGroup(ctx, tx, groupID)
	if err != nil {
		return err
	}
	if g == nil {
		return fmt.Errorf("group %q not found", groupID)
	}
	if err := groupsql.CheckVersion(g, meta); err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, `
		UPDATE registrations SET needs_coach = $3
		WHERE id = $1 AND group_id = $2 AND attendance_type = 'full_week'`,
		camperID, groupID, needsCoach)
	if err != nil {
		return fmt.Errorf("update camper coach: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("camper %q not in group %q", camperID, groupID)
	}
	extra := ""
	if clearCoachInvoice {
		extra = `, stripe_coach_invoice_id = NULL, coach_invoice_due_at = NULL`
	}
	if err := groupsql.Stamp(ctx, tx, groupID, meta, domain.BillingTransition{}, extra); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// DeleteGroupMeta permanently removes a registration group. Campers cascade-delete
// via FK. Version is checked before delete so stale dashboards cannot remove a
// concurrently modified group.

func (r *Repository) SetStripeCustomerID(ctx context.Context, groupID, customerID string) error {
	return groupsql.SetStripeCustomerID(ctx, r.pool, groupID, customerID)
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
