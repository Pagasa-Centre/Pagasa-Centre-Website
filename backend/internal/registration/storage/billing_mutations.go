package storage

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"pagasacentre/backend/internal/registration/domain"
)

// checkVersion returns domain.ErrVersionConflict if meta expects a version that no longer matches.
func checkVersion(g *domain.Group, meta domain.ActionMeta) error {
	if !meta.EnforceVersion() {
		return nil
	}
	if g == nil || g.Version != meta.ExpectedVersion {
		return domain.ErrVersionConflict
	}
	return nil
}

func stampExec(ctx context.Context, q pgx.Tx, groupID string, meta domain.ActionMeta, extraSQL string, extraArgs ...any) error {
	base := `UPDATE registration_groups
	 SET version = version + 1,
	     last_action = $1,
	     last_action_by = $2,
	     last_action_at = now()` + extraSQL
	args := []any{meta.Action, meta.Actor}
	args = append(args, extraArgs...)
	n := len(args) + 1
	args = append(args, groupID)
	where := fmt.Sprintf(` WHERE id = $%d`, n)
	if meta.EnforceVersion() {
		n++
		args = append(args, meta.ExpectedVersion)
		where += fmt.Sprintf(` AND version = $%d`, n)
	}
	tag, err := q.Exec(ctx, base+where, args...)
	if err != nil {
		return fmt.Errorf("stamp group: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrVersionConflict
	}
	return nil
}

func setCamperAllocationsTx(ctx context.Context, tx pgx.Tx, groupID string, allocs []CamperAllocation) error {
	for _, a := range allocs {
		var unitCode any
		if strings.TrimSpace(a.AllocatedUnitCode) != "" {
			unitCode = strings.TrimSpace(a.AllocatedUnitCode)
		}
		tag, err := tx.Exec(ctx,
			`UPDATE registrations
			    SET allocated_accommodation_code = $1,
			        allocated_unit_code = $2,
			        billed_stripe_price_id = $3
			  WHERE id = $4 AND group_id = $5`,
			a.AllocatedAccommodationCode, unitCode, a.BilledStripePriceID, a.CamperID, groupID)
		if err != nil {
			return fmt.Errorf("set allocation for camper %s: %w", a.CamperID, err)
		}
		if tag.RowsAffected() == 0 {
			return fmt.Errorf("camper %s not in group %s", a.CamperID, groupID)
		}
	}
	return nil
}

// AllocateGroup saves placements and sets billing_status=allocated atomically.
func (r *Repository) AllocateGroup(ctx context.Context, groupID string, allocs []CamperAllocation, meta domain.ActionMeta) error {
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
	if err := setCamperAllocationsTx(ctx, tx, groupID, allocs); err != nil {
		return err
	}
	if err := stampExec(ctx, tx, groupID, meta, `, billing_status = 'allocated'`); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// UnallocateGroup clears placements and returns billing_status to none.
func (r *Repository) UnallocateGroup(ctx context.Context, groupID string, meta domain.ActionMeta) error {
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
	_, err = tx.Exec(ctx,
		`UPDATE registrations
		    SET allocated_accommodation_code = NULL,
		        allocated_unit_code = NULL,
		        billed_stripe_price_id = NULL
		  WHERE group_id = $1`, groupID)
	if err != nil {
		return fmt.Errorf("clear allocations: %w", err)
	}
	if err := stampExec(ctx, tx, groupID, meta, `, billing_status = 'none'`); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// ConfirmFreeMeta marks a church-sponsored group as fully confirmed (no invoice).
func (r *Repository) ConfirmFreeMeta(ctx context.Context, groupID string, meta domain.ActionMeta) error {
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
	if !g.IsFree {
		return fmt.Errorf("group %q is not church-sponsored", groupID)
	}
	if g.BillingStatus != domain.BillingAllocated {
		return fmt.Errorf("group %q must be allocated before confirming sponsorship", groupID)
	}
	if err := checkVersion(g, meta); err != nil {
		return err
	}
	if err := stampExec(ctx, tx, groupID, meta, `, billing_status = 'free_confirmed'`); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// SetInvoiceDetailsMeta records invoice id/due and moves to invoiced with
// attribution. coachIncluded records whether the coach fee was folded into this
// balance invoice, so we never also send a separate coach invoice for the group.
func (r *Repository) SetInvoiceDetailsMeta(ctx context.Context, groupID, invoiceID string, dueAt time.Time, coachIncluded bool, meta domain.ActionMeta) error {
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
	extra := `, stripe_invoice_id = $3, invoice_due_at = $4, billing_status = 'invoiced', coach_included_in_balance = $5`
	if err := stampExec(ctx, tx, groupID, meta, extra, invoiceID, dueAt.UTC(), coachIncluded); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// SetCoachInvoiceMeta records a separate coach invoice id/due without touching
// billing_status (coach is tracked in parallel to the balance lifecycle).
func (r *Repository) SetCoachInvoiceMeta(ctx context.Context, groupID, coachInvoiceID string, dueAt time.Time, meta domain.ActionMeta) error {
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
	extra := `, stripe_coach_invoice_id = $3, coach_invoice_due_at = $4`
	if err := stampExec(ctx, tx, groupID, meta, extra, coachInvoiceID, dueAt.UTC()); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// MarkCoachFeePaidMeta stamps coach_fee_paid_at (Stripe webhook).
func (r *Repository) MarkCoachFeePaidMeta(ctx context.Context, groupID string, meta domain.ActionMeta) error {
	extra := `, coach_fee_paid_at = $3`
	return r.stampGroupPool(ctx, groupID, meta, extra, time.Now().UTC())
}

// WaiveCoachFeeMeta marks the coach fee waived and clears any separate coach invoice fields.
func (r *Repository) WaiveCoachFeeMeta(ctx context.Context, groupID string, meta domain.ActionMeta) error {
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
	now := time.Now().UTC()
	extra := `, coach_fee_waived_at = $3, stripe_coach_invoice_id = NULL, coach_invoice_due_at = NULL`
	if err := stampExec(ctx, tx, groupID, meta, extra, now); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// UnwaiveCoachFeeMeta clears coach_fee_waived_at so the group can be coach-charged again.
func (r *Repository) UnwaiveCoachFeeMeta(ctx context.Context, groupID string, meta domain.ActionMeta) error {
	extra := `, coach_fee_waived_at = NULL`
	return r.stampGroupPool(ctx, groupID, meta, extra)
}

// ClearInvoiceAndReleaseMeta voids invoice state, releases allocation, stamps attribution.
func (r *Repository) ClearInvoiceAndReleaseMeta(ctx context.Context, groupID string, meta domain.ActionMeta) error {
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
	_, err = tx.Exec(ctx,
		`UPDATE registrations
		    SET allocated_accommodation_code = NULL,
		        allocated_unit_code = NULL,
		        billed_stripe_price_id = NULL
		  WHERE group_id = $1`, groupID)
	if err != nil {
		return fmt.Errorf("clear camper allocations: %w", err)
	}
	extra := `, billing_status = 'released', stripe_invoice_id = NULL, invoice_due_at = NULL`
	if err := stampExec(ctx, tx, groupID, meta, extra); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// DeleteCamperMeta removes one camper row from a group. newBillingStatus, when
// non-empty, reverts the group's billing_status and clears invoice fields
// (used after voiding a now-stale invoice).
func (r *Repository) DeleteCamperMeta(ctx context.Context, groupID, camperID, newBillingStatus string, meta domain.ActionMeta) error {
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
	tag, err := tx.Exec(ctx, `DELETE FROM registrations WHERE id = $1 AND group_id = $2`, camperID, groupID)
	if err != nil {
		return fmt.Errorf("delete camper: %w", err)
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

// DayPassFields holds day-visitor details applied during a full-week conversion.
type DayPassFields struct {
	Days           []string
	TshirtOption   string
	ShirtSize      *string
	NeedsCatering  bool
	Dietary        *string
}

// ConvertCamperToDayPassMeta rewrites one camper as a day-visitor, clearing
// accommodation/coach/allocation and recording any deposit credit applied in Stripe.
func (r *Repository) ConvertCamperToDayPassMeta(
	ctx context.Context,
	groupID, camperID string,
	dp DayPassFields,
	depositCreditPence int,
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
			deposit_credit_pence = $8
		WHERE id = $1 AND group_id = $2`,
		camperID, groupID, dp.Days, dp.TshirtOption, dp.NeedsCatering, dp.ShirtSize, dp.Dietary, depositCreditPence)
	if err != nil {
		return fmt.Errorf("convert camper to day pass: %w", err)
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
	if err := stampExec(ctx, tx, groupID, meta, ""); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// DeleteGroupMeta permanently removes a registration group. Campers cascade-delete
// via FK. Version is checked before delete so stale dashboards cannot remove a
// concurrently modified group.
func (r *Repository) DeleteGroupMeta(ctx context.Context, groupID string, meta domain.ActionMeta) error {
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
	tag, err := tx.Exec(ctx, `DELETE FROM registration_groups WHERE id = $1`, groupID)
	if err != nil {
		return fmt.Errorf("delete group: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrVersionConflict
	}
	return tx.Commit(ctx)
}

// MarkBalancePaidMeta sets balance_paid with attribution (Stripe webhook).
func (r *Repository) MarkBalancePaidMeta(ctx context.Context, groupID string, meta domain.ActionMeta) error {
	extra := `, billing_status = 'balance_paid', balance_paid_at = $3`
	return r.stampGroupPool(ctx, groupID, meta, extra, time.Now().UTC())
}

// ExtendInvoiceDueAtMeta updates due date with optional version check.
func (r *Repository) ExtendInvoiceDueAtMeta(ctx context.Context, groupID string, dueAt time.Time, meta domain.ActionMeta) error {
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
	if g.BillingStatus != domain.BillingInvoiced {
		return fmt.Errorf("group %q not invoiced", groupID)
	}
	if err := checkVersion(g, meta); err != nil {
		return err
	}
	extra := `, invoice_due_at = $3`
	if err := stampExec(ctx, tx, groupID, meta, extra, dueAt.UTC()); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *Repository) stampGroupPool(ctx context.Context, groupID string, meta domain.ActionMeta, extraSQL string, extraArgs ...any) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if meta.EnforceVersion() {
		g, err := r.GetGroupByIDForUpdate(ctx, tx, groupID)
		if err != nil {
			return err
		}
		if err := checkVersion(g, meta); err != nil {
			return err
		}
	}
	if err := stampExec(ctx, tx, groupID, meta, extraSQL, extraArgs...); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// StampOnly bumps version/attribution without changing billing fields (e.g. resend).
func (r *Repository) StampOnly(ctx context.Context, groupID string, meta domain.ActionMeta) error {
	return r.stampGroupPool(ctx, groupID, meta, "")
}
