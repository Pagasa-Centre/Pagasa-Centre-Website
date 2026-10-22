package storage

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"pagasacentre/backend/internal/registration/domain"
	"pagasacentre/backend/internal/registration/storage/groupsql"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) FindGroupByStripeInvoiceID(ctx context.Context, invoiceID string) (*domain.Group, error) {
	const q = `SELECT ` + groupsql.GroupSelectCols + `
		  FROM registration_groups
		 WHERE stripe_invoice_id = $1`
	g, err := groupsql.ScanGroup(r.pool.QueryRow(ctx, q, invoiceID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find group by invoice: %w", err)
	}
	return &g, nil
}

func (r *Repository) FindGroupByStripeCoachInvoiceID(ctx context.Context, invoiceID string) (*domain.Group, error) {
	const q = `SELECT ` + groupsql.GroupSelectCols + `
		  FROM registration_groups
		 WHERE stripe_coach_invoice_id = $1`
	g, err := groupsql.ScanGroup(r.pool.QueryRow(ctx, q, invoiceID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find group by coach invoice: %w", err)
	}
	return &g, nil
}

func (r *Repository) MarkCoachFeePaidMeta(ctx context.Context, groupID string, meta domain.ActionMeta) error {
	extra := `, coach_fee_paid_at = $3`
	return groupsql.StampPool(ctx, r.pool, groupID, meta, domain.BillingTransition{}, extra, time.Now().UTC())
}

func (r *Repository) MarkBalancePaidMeta(ctx context.Context, groupID string, meta domain.ActionMeta, tr domain.BillingTransition) error {
	return groupsql.MarkBalancePaid(ctx, r.pool, groupID, meta, tr)
}
