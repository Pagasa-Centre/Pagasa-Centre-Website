package registration

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// checkVersion returns ErrVersionConflict if meta expects a version that no longer matches.
func checkVersion(g *Group, meta ActionMeta) error {
	if !meta.enforceVersion() {
		return nil
	}
	if g == nil || g.Version != meta.ExpectedVersion {
		return ErrVersionConflict
	}
	return nil
}

func stampExec(ctx context.Context, q pgx.Tx, groupID string, meta ActionMeta, extraSQL string, extraArgs ...any) error {
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
	if meta.enforceVersion() {
		n++
		args = append(args, meta.ExpectedVersion)
		where += fmt.Sprintf(` AND version = $%d`, n)
	}
	tag, err := q.Exec(ctx, base+where, args...)
	if err != nil {
		return fmt.Errorf("stamp group: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrVersionConflict
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
func (r *Repository) AllocateGroup(ctx context.Context, groupID string, allocs []CamperAllocation, meta ActionMeta) error {
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
func (r *Repository) UnallocateGroup(ctx context.Context, groupID string, meta ActionMeta) error {
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

// SetInvoiceDetailsMeta records invoice id/due and moves to invoiced with attribution.
func (r *Repository) SetInvoiceDetailsMeta(ctx context.Context, groupID, invoiceID string, dueAt time.Time, meta ActionMeta) error {
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
	extra := `, stripe_invoice_id = $3, invoice_due_at = $4, billing_status = 'invoiced'`
	if err := stampExec(ctx, tx, groupID, meta, extra, invoiceID, dueAt.UTC()); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// ClearInvoiceAndReleaseMeta voids invoice state, releases allocation, stamps attribution.
func (r *Repository) ClearInvoiceAndReleaseMeta(ctx context.Context, groupID string, meta ActionMeta) error {
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

// MarkBalancePaidMeta sets balance_paid with attribution (Stripe webhook).
func (r *Repository) MarkBalancePaidMeta(ctx context.Context, groupID string, meta ActionMeta) error {
	extra := `, billing_status = 'balance_paid', balance_paid_at = $3`
	return r.stampGroupPool(ctx, groupID, meta, extra, time.Now().UTC())
}

// ExtendInvoiceDueAtMeta updates due date with optional version check.
func (r *Repository) ExtendInvoiceDueAtMeta(ctx context.Context, groupID string, dueAt time.Time, meta ActionMeta) error {
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
	if g.BillingStatus != BillingInvoiced {
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

func (r *Repository) stampGroupPool(ctx context.Context, groupID string, meta ActionMeta, extraSQL string, extraArgs ...any) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if meta.enforceVersion() {
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
func (r *Repository) StampOnly(ctx context.Context, groupID string, meta ActionMeta) error {
	return r.stampGroupPool(ctx, groupID, meta, "")
}
