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

func (r *Repository) AllocateGroup(ctx context.Context, groupID string, allocs []domain.CamperAllocation, meta domain.ActionMeta, tr domain.BillingTransition) error {
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
	if err := groupsql.SetCamperAllocationsTx(ctx, tx, groupID, allocs); err != nil {
		return err
	}
	if err := groupsql.Stamp(ctx, tx, groupID, meta, tr, ""); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// UnallocateGroup clears placements and returns billing_status to none.

func (r *Repository) UnallocateGroup(ctx context.Context, groupID string, meta domain.ActionMeta, tr domain.BillingTransition) error {
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
	_, err = tx.Exec(ctx,
		`UPDATE registrations
		    SET allocated_accommodation_code = NULL,
		        allocated_unit_code = NULL,
		        billed_stripe_price_id = NULL
		  WHERE group_id = $1`, groupID)
	if err != nil {
		return fmt.Errorf("clear allocations: %w", err)
	}
	if err := groupsql.Stamp(ctx, tx, groupID, meta, tr, ""); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// ConfirmFreeMeta marks a church-sponsored group as fully confirmed (no invoice).

func (r *Repository) ConfirmFreeMeta(ctx context.Context, groupID string, meta domain.ActionMeta, tr domain.BillingTransition) error {
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
	if err := groupsql.Stamp(ctx, tx, groupID, meta, tr, ""); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// SetInvoiceDetailsMeta records invoice id/due and moves to invoiced with
// attribution. coachIncluded records whether the coach fee was folded into this
// balance invoice, so we never also send a separate coach invoice for the group.

func (r *Repository) ClearInvoiceAndReleaseMeta(ctx context.Context, groupID string, meta domain.ActionMeta, tr domain.BillingTransition) error {
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
	_, err = tx.Exec(ctx,
		`UPDATE registrations
		    SET allocated_accommodation_code = NULL,
		        allocated_unit_code = NULL,
		        billed_stripe_price_id = NULL
		  WHERE group_id = $1`, groupID)
	if err != nil {
		return fmt.Errorf("clear camper allocations: %w", err)
	}
	extra := `, stripe_invoice_id = NULL, invoice_due_at = NULL`
	if err := groupsql.Stamp(ctx, tx, groupID, meta, tr, extra); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// DeleteCamperMeta removes one camper row from a group.
