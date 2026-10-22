package storage

import (
	"context"
	"fmt"
	"time"

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

func (r *Repository) SetInvoiceDetailsMeta(ctx context.Context, groupID, invoiceID string, dueAt time.Time, coachIncluded bool, meta domain.ActionMeta, tr domain.BillingTransition) error {
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
	extra := `, stripe_invoice_id = $3, invoice_due_at = $4, coach_included_in_balance = $5`
	if err := groupsql.Stamp(ctx, tx, groupID, meta, tr, extra, invoiceID, dueAt.UTC(), coachIncluded); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// SetCoachInvoiceMeta records a separate coach invoice id/due without touching
// billing_status (coach is tracked in parallel to the balance lifecycle).

func (r *Repository) SetCoachInvoiceMeta(ctx context.Context, groupID, coachInvoiceID string, dueAt time.Time, meta domain.ActionMeta) error {
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
	extra := `, stripe_coach_invoice_id = $3, coach_invoice_due_at = $4`
	if err := groupsql.Stamp(ctx, tx, groupID, meta, domain.BillingTransition{}, extra, coachInvoiceID, dueAt.UTC()); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// WaiveCoachFeeMeta marks the coach fee waived and clears any separate coach invoice fields.
func (r *Repository) WaiveCoachFeeMeta(ctx context.Context, groupID string, meta domain.ActionMeta) error {
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
	now := time.Now().UTC()
	extra := `, coach_fee_waived_at = $3, stripe_coach_invoice_id = NULL, coach_invoice_due_at = NULL`
	if err := groupsql.Stamp(ctx, tx, groupID, meta, domain.BillingTransition{}, extra, now); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// UnwaiveCoachFeeMeta clears coach_fee_waived_at so the group can be coach-charged again.

func (r *Repository) UnwaiveCoachFeeMeta(ctx context.Context, groupID string, meta domain.ActionMeta) error {
	extra := `, coach_fee_waived_at = NULL`
	return groupsql.StampPool(ctx, r.pool, groupID, meta, domain.BillingTransition{}, extra)
}

// ClearInvoiceAndReleaseMeta voids invoice state, releases allocation, stamps attribution.

func (r *Repository) DeleteGroupMeta(ctx context.Context, groupID string, meta domain.ActionMeta) error {
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
	tag, err := tx.Exec(ctx, `DELETE FROM registration_groups WHERE id = $1`, groupID)
	if err != nil {
		return fmt.Errorf("delete group: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrVersionConflict
	}
	return tx.Commit(ctx)
}

// MarkBalancePaidMeta sets balance_paid with attribution. Both settlement routes
// land here — the Stripe webhook when the family pays their invoice, and the
// manual bank-transfer button — so any deposit owed by a late arrival is settled
// in the same transaction: it moves out of what they owe and into what they have
// paid. Clearing what is owed keeps a settled deposit off any later invoice;
// recording what was paid is what lets a delete keep that deposit rather than
// hand it back inside the balance refund.
//
// This is the only place a deposit becomes paid. Waiving a deposit and converting
// a camper to a day visitor also zero what is owed, but no money arrives, so
// neither may write the paid column.

func (r *Repository) ExtendInvoiceDueAtMeta(ctx context.Context, groupID string, dueAt time.Time, meta domain.ActionMeta, tr domain.BillingTransition) error {
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
	extra := `, invoice_due_at = $3`
	if err := groupsql.Stamp(ctx, tx, groupID, meta, tr, extra, dueAt.UTC()); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *Repository) StampOnly(ctx context.Context, groupID string, meta domain.ActionMeta, tr domain.BillingTransition) error {
	return groupsql.StampOnly(ctx, r.pool, groupID, meta, tr)
}

func (r *Repository) MarkBalancePaidMeta(ctx context.Context, groupID string, meta domain.ActionMeta, tr domain.BillingTransition) error {
	return groupsql.MarkBalancePaid(ctx, r.pool, groupID, meta, tr)
}

func (r *Repository) SetStripeCustomerID(ctx context.Context, groupID, customerID string) error {
	return groupsql.SetStripeCustomerID(ctx, r.pool, groupID, customerID)
}
