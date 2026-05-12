package accommodation

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

// ListWithAvailability returns every accommodation type plus a count of campers
// from groups whose payment_status is 'paid'.
func (r *Repository) ListWithAvailability(ctx context.Context) ([]Availability, error) {
	const q = `
		SELECT at.code, at.display_name, at.capacity, at.sort_order, COALESCE(at.notes, ''),
		       COALESCE(t.taken, 0) AS taken
		  FROM accommodation_types at
		  LEFT JOIN (
		      SELECT r.accommodation_code, COUNT(*) AS taken
		        FROM registrations r
		        JOIN registration_groups g ON g.id = r.group_id
		       WHERE g.payment_status = 'paid' AND r.accommodation_code IS NOT NULL
		    GROUP BY r.accommodation_code
		  ) t ON t.accommodation_code = at.code
		 ORDER BY at.sort_order, at.code`
	rows, err := r.pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list accommodations: %w", err)
	}
	defer rows.Close()

	var out []Availability
	for rows.Next() {
		var a Availability
		var sortOrder int
		if err := rows.Scan(&a.Code, &a.DisplayName, &a.Capacity, &sortOrder, &a.Notes, &a.Taken); err != nil {
			return nil, fmt.Errorf("scan accommodation: %w", err)
		}
		if a.Capacity != nil {
			rem := *a.Capacity - a.Taken
			if rem < 0 {
				rem = 0
			}
			a.Remaining = &rem
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// LockAndCount locks the accommodation_types row for the given code and returns
// its capacity plus the current count of paid campers assigned to it. Must be
// called inside a transaction.
func (r *Repository) LockAndCount(ctx context.Context, tx pgx.Tx, code string) (capacity *int, taken int, err error) {
	const lockSQL = `SELECT capacity FROM accommodation_types WHERE code = $1 FOR UPDATE`
	if err = tx.QueryRow(ctx, lockSQL, code).Scan(&capacity); err != nil {
		return nil, 0, fmt.Errorf("lock accommodation %s: %w", code, err)
	}
	const countSQL = `
		SELECT COUNT(*) FROM registrations r
		  JOIN registration_groups g ON g.id = r.group_id
		 WHERE r.accommodation_code = $1 AND g.payment_status = 'paid'`
	if err = tx.QueryRow(ctx, countSQL, code).Scan(&taken); err != nil {
		return nil, 0, fmt.Errorf("count paid accommodations: %w", err)
	}
	return capacity, taken, nil
}

// UpdateCapacity sets the capacity for a code (nil means unlimited).
func (r *Repository) UpdateCapacity(ctx context.Context, code string, capacity *int) error {
	tag, err := r.pool.Exec(ctx, `UPDATE accommodation_types SET capacity = $1 WHERE code = $2`, capacity, code)
	if err != nil {
		return fmt.Errorf("update capacity: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("accommodation %q not found", code)
	}
	return nil
}
