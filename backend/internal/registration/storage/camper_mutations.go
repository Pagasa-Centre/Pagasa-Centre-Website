package storage

import (
	"context"
	"fmt"

	"pagasacentre/backend/internal/registration/domain"
)

// CamperDetails is the full set of identity and stay fields an admin can rewrite
// on an existing camper. Every field arrives on every edit and replaces what is
// stored; there is no merging, so a rule can never be skipped by omitting a field.
//
// The full-week fields are only applied to full-week campers. A day visitor's
// stay details stay with the day-pass editor so no field has two owners.
type CamperDetails struct {
	FirstName      string
	LastName       string
	Gender         string
	Age            int
	CellLeaderName string
	IsCellLeader   bool

	ShirtSize                  *string
	DietaryRequirements        *string
	AccommodationFirstChoice   *string
	AccommodationSecondChoice  *string
	AllocatedAccommodationCode *string
	AllocatedUnitCode          *string
	BilledStripePriceID        *string
}

// UpdateCamperDetailsMeta rewrites one camper's details. newBillingStatus, when
// non-empty, reverts the group's billing_status and clears invoice fields after a
// now-stale invoice has been voided.
func (r *Repository) UpdateCamperDetailsMeta(
	ctx context.Context,
	groupID, camperID string,
	d CamperDetails,
	newBillingStatus string,
	meta domain.ActionMeta,
) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	g, err := r.GetGroupByIDForUpdate(ctx, tx, groupID)
	if err != nil {
		return err
	}
	if g == nil {
		return fmt.Errorf("group %q not found", groupID)
	}
	if err := checkVersion(g, meta); err != nil {
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

	if newBillingStatus == "" {
		if err := stampExec(ctx, tx, groupID, meta, ""); err != nil {
			return err
		}
	} else {
		extra := `, billing_status = $3, stripe_invoice_id = NULL, invoice_due_at = NULL`
		if err := stampExec(ctx, tx, groupID, meta, extra, newBillingStatus); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// AddCamperMeta inserts a camper into an existing group and returns the new row's
// id. depositOwedPence records a deposit this person still owes because the
// group's deposit checkout was already settled before they joined.
// newBillingStatus, when non-empty, reverts the group's billing_status and clears
// invoice fields.
func (r *Repository) AddCamperMeta(
	ctx context.Context,
	groupID string,
	c domain.CamperDTO,
	depositOwedPence int,
	newBillingStatus string,
	meta domain.ActionMeta,
) (string, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)

	g, err := r.GetGroupByIDForUpdate(ctx, tx, groupID)
	if err != nil {
		return "", err
	}
	if g == nil {
		return "", fmt.Errorf("group %q not found", groupID)
	}
	if err := checkVersion(g, meta); err != nil {
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

	if newBillingStatus == "" {
		if err := stampExec(ctx, tx, groupID, meta, ""); err != nil {
			return "", err
		}
	} else {
		extra := `, billing_status = $3, stripe_invoice_id = NULL, invoice_due_at = NULL`
		if err := stampExec(ctx, tx, groupID, meta, extra, newBillingStatus); err != nil {
			return "", err
		}
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
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	g, err := r.GetGroupByIDForUpdate(ctx, tx, groupID)
	if err != nil {
		return err
	}
	if g == nil {
		return fmt.Errorf("group %q not found", groupID)
	}
	if err := checkVersion(g, meta); err != nil {
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
	if err := stampExec(ctx, tx, groupID, meta, ""); err != nil {
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
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	g, err := r.GetGroupByIDForUpdate(ctx, tx, groupID)
	if err != nil {
		return err
	}
	if g == nil {
		return fmt.Errorf("group %q not found", groupID)
	}
	if err := checkVersion(g, meta); err != nil {
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
	if err := stampExec(ctx, tx, groupID, meta, ""); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
