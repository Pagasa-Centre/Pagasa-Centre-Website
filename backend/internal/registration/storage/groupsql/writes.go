package groupsql

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"pagasacentre/backend/internal/registration/domain"
	commondb "pagasacentre/backend/pkg/commonlibrary/db"
)

func StampOnly(ctx context.Context, pool *pgxpool.Pool, groupID string, meta domain.ActionMeta, tr domain.BillingTransition) error {
	return StampPool(ctx, pool, groupID, meta, tr, "")
}

func SetBillingCustomerRef(ctx context.Context, pool *pgxpool.Pool, groupID, customerID string) error {
	_, err := commondb.Conn(ctx, pool).Exec(ctx,
		`UPDATE registration_groups SET stripe_customer_id = $1 WHERE id = $2`,
		customerID, groupID)
	if err != nil {
		return fmt.Errorf("set stripe_customer_id: %w", err)
	}
	return nil
}

func MarkBalancePaid(ctx context.Context, pool *pgxpool.Pool, groupID string, meta domain.ActionMeta, tr domain.BillingTransition) error {
	tx, err := commondb.Begin(ctx, pool)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if meta.EnforceVersion() {
		g, err := LockGroup(ctx, tx, groupID)
		if err != nil {
			return err
		}
		if err := CheckVersion(g, meta); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(ctx,
		`UPDATE registrations
		    SET deposit_paid_pence = deposit_paid_pence + deposit_owed_pence,
		        deposit_owed_pence = 0
		  WHERE group_id = $1 AND deposit_owed_pence > 0`, groupID); err != nil {
		return fmt.Errorf("settle outstanding deposits: %w", err)
	}
	extra := `, balance_paid_at = $3`
	if err := Stamp(ctx, tx, groupID, meta, tr, extra, time.Now().UTC()); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
