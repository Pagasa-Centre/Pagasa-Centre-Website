package storage

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"pagasacentre/backend/internal/registration/domain"
)

var (
	ErrFreeCodeInvalid      = errors.New("free code invalid")
	ErrFreeCodeNotRevocable = errors.New("free code not revocable")
)

// ReserveFreeCode locks an unused, non-revoked code row for redemption inside tx.
func (r *Repository) ReserveFreeCode(ctx context.Context, tx pgx.Tx, code string) (string, error) {
	const q = `
		SELECT id FROM free_codes
		 WHERE code = $1 AND used_at IS NULL AND revoked_at IS NULL
		 FOR UPDATE`
	var id string
	err := tx.QueryRow(ctx, q, code).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrFreeCodeInvalid
	}
	if err != nil {
		return "", fmt.Errorf("reserve free code: %w", err)
	}
	return id, nil
}

// MarkFreeCodeUsed records redemption on a reserved code row.
func (r *Repository) MarkFreeCodeUsed(ctx context.Context, tx pgx.Tx, codeID, groupID string) error {
	tag, err := tx.Exec(ctx,
		`UPDATE free_codes SET used_at = now(), used_by_group_id = $2 WHERE id = $1`,
		codeID, groupID)
	if err != nil {
		return fmt.Errorf("mark free code used: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrFreeCodeInvalid
	}
	return nil
}

// GenerateFreeCode inserts a new unused code (pool-level, no tx required).
func (r *Repository) GenerateFreeCode(ctx context.Context, code, createdBy, note string) error {
	var noteVal any
	if note != "" {
		noteVal = note
	}
	_, err := r.pool.Exec(ctx,
		`INSERT INTO free_codes (code, created_by, note) VALUES ($1, $2, $3)`,
		code, createdBy, noteVal)
	if err != nil {
		return fmt.Errorf("insert free code: %w", err)
	}
	return nil
}

// ListFreeCodes returns all codes newest first.
func (r *Repository) ListFreeCodes(ctx context.Context) ([]domain.FreeCode, error) {
	const q = `
		SELECT id, code, created_at, created_by, note, used_at, used_by_group_id, revoked_at
		  FROM free_codes
		 ORDER BY created_at DESC`
	rows, err := r.pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list free codes: %w", err)
	}
	defer rows.Close()
	var out []domain.FreeCode
	for rows.Next() {
		var fc domain.FreeCode
		if err := rows.Scan(
			&fc.ID, &fc.Code, &fc.CreatedAt, &fc.CreatedBy, &fc.Note,
			&fc.UsedAt, &fc.UsedByGroupID, &fc.RevokedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, fc)
	}
	return out, rows.Err()
}

// RevokeFreeCode soft-disables an unused code.
func (r *Repository) RevokeFreeCode(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE free_codes SET revoked_at = now()
		  WHERE id = $1 AND used_at IS NULL AND revoked_at IS NULL`,
		id)
	if err != nil {
		return fmt.Errorf("revoke free code: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrFreeCodeNotRevocable
	}
	return nil
}
