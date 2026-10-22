package storage

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"pagasacentre/backend/internal/registration/domain"
)

// ListWithBilling is List with optional billing_status filter.
func (r *Repository) ListWithBilling(ctx context.Context, f domain.ListFilterBilling) ([]domain.Group, error) {
	args := []any{}
	q := `SELECT ` + groupSelectCols + ` FROM registration_groups WHERE 1=1`
	n := 1
	if f.PaymentStatus != "" {
		q += fmt.Sprintf(` AND payment_status = $%d`, n)
		args = append(args, f.PaymentStatus)
		n++
	}
	if f.BillingStatus != "" {
		q += fmt.Sprintf(` AND billing_status = $%d`, n)
		args = append(args, f.BillingStatus)
		n++
	}
	q += ` ORDER BY created_at DESC`

	rows, err := r.db(ctx).Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("list groups: %w", err)
	}
	defer rows.Close()

	var out []domain.Group
	for rows.Next() {
		g, err := scanGroup(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

// GetAccommodationType loads a tier including its Stripe Price id.
func (r *Repository) GetAccommodationType(ctx context.Context, code string) (*domain.AccommodationType, error) {
	const q = `SELECT code, display_name, capacity, stripe_price_id, available_for_registration FROM accommodation_types WHERE code = $1`
	var t domain.AccommodationType
	var capacity *int
	var priceID *string
	err := r.db(ctx).QueryRow(ctx, q, code).Scan(&t.Code, &t.DisplayName, &capacity, &priceID, &t.AvailableForRegistration)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get accommodation type: %w", err)
	}
	t.Capacity = capacity
	t.StripePriceID = priceID
	return &t, nil
}

// ListAccommodationTypes returns all tiers with capacity and Stripe Price ids.
func (r *Repository) ListAccommodationTypes(ctx context.Context) ([]domain.AccommodationType, error) {
	const q = `SELECT code, display_name, capacity, stripe_price_id, available_for_registration FROM accommodation_types ORDER BY sort_order, code`
	rows, err := r.db(ctx).Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list accommodation types: %w", err)
	}
	defer rows.Close()

	var out []domain.AccommodationType
	for rows.Next() {
		var t domain.AccommodationType
		var capacity *int
		var priceID *string
		if err := rows.Scan(&t.Code, &t.DisplayName, &capacity, &priceID, &t.AvailableForRegistration); err != nil {
			return nil, err
		}
		t.Capacity = capacity
		t.StripePriceID = priceID
		out = append(out, t)
	}
	return out, rows.Err()
}

// AccommodationAvailability returns a map of code -> available_for_registration.
func (r *Repository) AccommodationAvailability(ctx context.Context) (map[string]bool, error) {
	const q = `SELECT code, available_for_registration FROM accommodation_types`
	rows, err := r.db(ctx).Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("accommodation availability: %w", err)
	}
	defer rows.Close()

	out := make(map[string]bool)
	for rows.Next() {
		var code string
		var available bool
		if err := rows.Scan(&code, &available); err != nil {
			return nil, err
		}
		out[code] = available
	}
	return out, rows.Err()
}

// SetAccommodationAvailableForRegistration toggles a tier's public availability.
func (r *Repository) SetAccommodationAvailableForRegistration(ctx context.Context, code string, available bool) error {
	tag, err := r.db(ctx).Exec(ctx,
		`UPDATE accommodation_types SET available_for_registration = $1 WHERE code = $2`,
		available, code)
	if err != nil {
		return fmt.Errorf("set accommodation availability: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("accommodation %q not found", code)
	}
	return nil
}

// ListAccommodationUnits returns all physical units for admin allocation.
func (r *Repository) ListAccommodationUnits(ctx context.Context) ([]domain.AccommodationUnit, error) {
	const q = `SELECT code, accommodation_code, display_name, capacity, sort_order
		  FROM accommodation_units
		 ORDER BY accommodation_code, sort_order, code`
	rows, err := r.db(ctx).Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list accommodation units: %w", err)
	}
	defer rows.Close()

	var out []domain.AccommodationUnit
	for rows.Next() {
		var u domain.AccommodationUnit
		if err := rows.Scan(&u.Code, &u.AccommodationCode, &u.DisplayName, &u.Capacity, &u.SortOrder); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// GetAccommodationUnit loads a single unit by code.
func (r *Repository) GetAccommodationUnit(ctx context.Context, code string) (*domain.AccommodationUnit, error) {
	const q = `SELECT code, accommodation_code, display_name, capacity, sort_order
		  FROM accommodation_units WHERE code = $1`
	var u domain.AccommodationUnit
	err := r.db(ctx).QueryRow(ctx, q, code).Scan(
		&u.Code, &u.AccommodationCode, &u.DisplayName, &u.Capacity, &u.SortOrder)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get accommodation unit: %w", err)
	}
	return &u, nil
}

// UpdateAccommodationStripePrice sets stripe_price_id for a tier (admin/setup).
func (r *Repository) UpdateAccommodationStripePrice(ctx context.Context, code, priceID string) error {
	tag, err := r.db(ctx).Exec(ctx,
		`UPDATE accommodation_types SET stripe_price_id = $1 WHERE code = $2`,
		priceID, code)
	if err != nil {
		return fmt.Errorf("update accommodation stripe price: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("accommodation %q not found", code)
	}
	return nil
}
