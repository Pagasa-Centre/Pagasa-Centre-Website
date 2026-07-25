package storage

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"pagasacentre/backend/internal/camp/domain"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

func (r *Repository) GetConfig(ctx context.Context) (domain.Config, error) {
	var c domain.Config
	err := r.pool.QueryRow(ctx,
		`SELECT name, location_name, location_addr, website_url, start_date, end_date, registrations_open, registration_payment_mode
		   FROM camp_config WHERE id = 1`).
		Scan(&c.Name, &c.LocationName, &c.LocationAddr, &c.WebsiteURL,
			&c.StartDate, &c.EndDate, &c.RegistrationsOpen, &c.RegistrationPaymentMode)
	if err != nil {
		return domain.Config{}, fmt.Errorf("get camp config: %w", err)
	}
	return c, nil
}

func (r *Repository) RegistrationsOpen(ctx context.Context) (bool, error) {
	var open bool
	err := r.pool.QueryRow(ctx, `SELECT registrations_open FROM camp_config WHERE id = 1`).Scan(&open)
	if err != nil {
		return false, fmt.Errorf("read registrations_open: %w", err)
	}
	return open, nil
}

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

func (r *Repository) RegistrationPaymentMode(ctx context.Context) (string, error) {
	var mode string
	err := r.pool.QueryRow(ctx, `SELECT registration_payment_mode FROM camp_config WHERE id = 1`).Scan(&mode)
	if err != nil {
		return "", fmt.Errorf("read registration_payment_mode: %w", err)
	}
	return mode, nil
}

func (r *Repository) SetRegistrationPaymentMode(ctx context.Context, mode string) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE camp_config SET registration_payment_mode = $1 WHERE id = 1`, mode)
	if err != nil {
		return fmt.Errorf("set registration_payment_mode: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("camp_config row not found")
	}
	return nil
}

func (r *Repository) ListPrices(ctx context.Context) ([]domain.Price, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT code, display_name, amount_pence, currency FROM prices ORDER BY code`)
	if err != nil {
		return nil, fmt.Errorf("list prices: %w", err)
	}
	defer rows.Close()
	var out []domain.Price
	for rows.Next() {
		var p domain.Price
		if err := rows.Scan(&p.Code, &p.DisplayName, &p.AmountPence, &p.Currency); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (r *Repository) GetPrice(ctx context.Context, code string) (domain.Price, error) {
	var p domain.Price
	err := r.pool.QueryRow(ctx,
		`SELECT code, display_name, amount_pence, currency FROM prices WHERE code = $1`, code).
		Scan(&p.Code, &p.DisplayName, &p.AmountPence, &p.Currency)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Price{}, fmt.Errorf("price %q not found", code)
	}
	if err != nil {
		return domain.Price{}, fmt.Errorf("get price: %w", err)
	}
	return p, nil
}

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
