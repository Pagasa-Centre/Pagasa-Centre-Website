package groupsql

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"pagasacentre/backend/internal/registration/domain"
	commondb "pagasacentre/backend/pkg/commonlibrary/db"
)

func Stamp(ctx context.Context, q pgx.Tx, groupID string, meta domain.ActionMeta, tr domain.BillingTransition, extraSQL string, extraArgs ...any) error {
	set := `UPDATE registration_groups
	 SET version = version + 1,
	     last_action = $1,
	     last_action_by = $2,
	     last_action_at = now()`
	args := []any{meta.Action, meta.Actor}
	set += extraSQL
	args = append(args, extraArgs...)
	if tr.ChangesBilling() {
		set += fmt.Sprintf(`, billing_status = $%d`, len(args)+1)
		args = append(args, tr.To)
	}

	args = append(args, groupID)
	where := fmt.Sprintf(` WHERE id = $%d`, len(args))
	if tr.From != "" {
		args = append(args, tr.From)
		where += fmt.Sprintf(` AND billing_status = $%d`, len(args))
	}
	if meta.EnforceVersion() {
		args = append(args, meta.ExpectedVersion)
		where += fmt.Sprintf(` AND version = $%d`, len(args))
	}
	tag, err := q.Exec(ctx, set+where, args...)
	if err != nil {
		return fmt.Errorf("stamp group: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrVersionConflict
	}
	return nil
}

func StampPool(ctx context.Context, pool *pgxpool.Pool, groupID string, meta domain.ActionMeta, tr domain.BillingTransition, extraSQL string, extraArgs ...any) error {
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
	if err := Stamp(ctx, tx, groupID, meta, tr, extraSQL, extraArgs...); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
