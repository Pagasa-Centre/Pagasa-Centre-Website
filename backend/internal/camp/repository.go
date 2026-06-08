package camp

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

func (r *Repository) GetConfig(ctx context.Context) (Config, error) {
	var c Config
	err := r.pool.QueryRow(ctx,
		`SELECT name, location_name, location_addr, website_url, start_date, end_date, registrations_open
		   FROM camp_config WHERE id = 1`).
		Scan(&c.Name, &c.LocationName, &c.LocationAddr, &c.WebsiteURL,
			&c.StartDate, &c.EndDate, &c.RegistrationsOpen)
	if err != nil {
		return Config{}, fmt.Errorf("get camp config: %w", err)
	}
	return c, nil
}

// RegistrationsOpen is a fast read of just the registrations_open flag.
func (r *Repository) RegistrationsOpen(ctx context.Context) (bool, error) {
	var open bool
	err := r.pool.QueryRow(ctx, `SELECT registrations_open FROM camp_config WHERE id = 1`).Scan(&open)
	if err != nil {
		return false, fmt.Errorf("read registrations_open: %w", err)
	}
	return open, nil
}

// SetRegistrationsOpen flips the public registration window on or off. Used by
// the admin dashboard so the White Team can open/close registration themselves.
func (r *Repository) SetRegistrationsOpen(ctx context.Context, open bool) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE camp_config SET registrations_open = $1 WHERE id = 1`, open)
	if err != nil {
		return fmt.Errorf("set registrations_open: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("camp_config row not found")
	}
	return nil
}

func (r *Repository) ListPrices(ctx context.Context) ([]Price, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT code, display_name, amount_pence, currency FROM prices ORDER BY code`)
	if err != nil {
		return nil, fmt.Errorf("list prices: %w", err)
	}
	defer rows.Close()
	var out []Price
	for rows.Next() {
		var p Price
		if err := rows.Scan(&p.Code, &p.DisplayName, &p.AmountPence, &p.Currency); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// GetPrice fetches a single price row by code.
func (r *Repository) GetPrice(ctx context.Context, code string) (Price, error) {
	var p Price
	err := r.pool.QueryRow(ctx,
		`SELECT code, display_name, amount_pence, currency FROM prices WHERE code = $1`, code).
		Scan(&p.Code, &p.DisplayName, &p.AmountPence, &p.Currency)
	if errors.Is(err, pgx.ErrNoRows) {
		return Price{}, fmt.Errorf("price %q not found", code)
	}
	if err != nil {
		return Price{}, fmt.Errorf("get price: %w", err)
	}
	return p, nil
}

// UpdatePrice sets a new amount_pence for an existing code.
func (r *Repository) UpdatePrice(ctx context.Context, code string, amountPence int) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE prices SET amount_pence = $1 WHERE code = $2`, amountPence, code)
	if err != nil {
		return fmt.Errorf("update price: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("price %q not found", code)
	}
	return nil
}
