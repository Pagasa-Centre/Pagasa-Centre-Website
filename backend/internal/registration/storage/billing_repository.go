package storage

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

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

	rows, err := r.pool.Query(ctx, q, args...)
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

// GetGroupByIDForUpdate locks a group row for billing mutations.
func (r *Repository) GetGroupByIDForUpdate(ctx context.Context, tx pgx.Tx, groupID string) (*domain.Group, error) {
	const q = `SELECT ` + groupSelectCols + `
		  FROM registration_groups
		 WHERE id = $1
		   FOR UPDATE`
	g, err := scanGroup(tx.QueryRow(ctx, q, groupID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get group for update: %w", err)
	}
	return &g, nil
}

// FindGroupByStripeInvoiceID looks up a group by its balance invoice id.
func (r *Repository) FindGroupByStripeInvoiceID(ctx context.Context, invoiceID string) (*domain.Group, error) {
	const q = `SELECT ` + groupSelectCols + `
		  FROM registration_groups
		 WHERE stripe_invoice_id = $1`
	g, err := scanGroup(r.pool.QueryRow(ctx, q, invoiceID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find group by invoice: %w", err)
	}
	return &g, nil
}

// CamperAllocation is one camper's White Team placement.
type CamperAllocation struct {
	CamperID                   string
	AllocatedAccommodationCode string
	AllocatedUnitCode          string
	BilledStripePriceID        string
}

// SetCamperAllocations updates allocation fields for campers in a group.
func (r *Repository) SetCamperAllocations(ctx context.Context, groupID string, allocs []CamperAllocation) error {
	for _, a := range allocs {
		var unitCode any
		if strings.TrimSpace(a.AllocatedUnitCode) != "" {
			unitCode = strings.TrimSpace(a.AllocatedUnitCode)
		}
		tag, err := r.pool.Exec(ctx,
			`UPDATE registrations
			    SET allocated_accommodation_code = $1,
			        allocated_unit_code = $2,
			        billed_stripe_price_id = $3
			  WHERE id = $4 AND group_id = $5`,
			a.AllocatedAccommodationCode, unitCode, a.BilledStripePriceID, a.CamperID, groupID)
		if err != nil {
			return fmt.Errorf("set allocation for camper %s: %w", a.CamperID, err)
		}
		if tag.RowsAffected() == 0 {
			return fmt.Errorf("camper %s not in group %s", a.CamperID, groupID)
		}
	}
	return nil
}

// ClearCamperAllocations wipes allocation fields for all campers in a group.
func (r *Repository) ClearCamperAllocations(ctx context.Context, groupID string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE registrations
		    SET allocated_accommodation_code = NULL,
		        allocated_unit_code = NULL,
		        billed_stripe_price_id = NULL
		  WHERE group_id = $1`, groupID)
	if err != nil {
		return fmt.Errorf("clear allocations: %w", err)
	}
	return nil
}

// SetBillingStatus updates billing_status on a group.
func (r *Repository) SetBillingStatus(ctx context.Context, groupID, status string) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE registration_groups SET billing_status = $1 WHERE id = $2`,
		status, groupID)
	if err != nil {
		return fmt.Errorf("set billing_status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("group %q not found", groupID)
	}
	return nil
}

// SetStripeCustomerID stores the Stripe Customer id for balance invoicing.
func (r *Repository) SetStripeCustomerID(ctx context.Context, groupID, customerID string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE registration_groups SET stripe_customer_id = $1 WHERE id = $2`,
		customerID, groupID)
	if err != nil {
		return fmt.Errorf("set stripe_customer_id: %w", err)
	}
	return nil
}

// SetInvoiceDetails records invoice id, due date, and moves to invoiced.
func (r *Repository) SetInvoiceDetails(ctx context.Context, groupID, invoiceID string, dueAt time.Time) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE registration_groups
		    SET stripe_invoice_id = $1,
		        invoice_due_at = $2,
		        billing_status = 'invoiced'
		  WHERE id = $3`,
		invoiceID, dueAt.UTC(), groupID)
	if err != nil {
		return fmt.Errorf("set invoice details: %w", err)
	}
	return nil
}

// MarkBalancePaid sets billing_status and balance_paid_at.
func (r *Repository) MarkBalancePaid(ctx context.Context, groupID string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE registration_groups
		    SET billing_status = 'balance_paid',
		        balance_paid_at = $1
		  WHERE id = $2`,
		time.Now().UTC(), groupID)
	if err != nil {
		return fmt.Errorf("mark balance paid: %w", err)
	}
	return nil
}

// ClearInvoiceAndRelease clears invoice fields, sets released, clears allocations.
func (r *Repository) ClearInvoiceAndRelease(ctx context.Context, groupID string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx,
		`UPDATE registration_groups
		    SET billing_status = 'released',
		        stripe_invoice_id = NULL,
		        invoice_due_at = NULL
		  WHERE id = $1`, groupID)
	if err != nil {
		return fmt.Errorf("release group: %w", err)
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
	return tx.Commit(ctx)
}

// ListOverdueInvoiced returns groups past invoice_due_at still in invoiced status.
func (r *Repository) ListOverdueInvoiced(ctx context.Context, now time.Time) ([]domain.Group, error) {
	const q = `SELECT ` + groupSelectCols + `
		  FROM registration_groups
		 WHERE billing_status = 'invoiced'
		   AND invoice_due_at IS NOT NULL
		   AND invoice_due_at < $1`
	rows, err := r.pool.Query(ctx, q, now.UTC())
	if err != nil {
		return nil, fmt.Errorf("list overdue invoiced: %w", err)
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
	const q = `SELECT code, display_name, capacity, stripe_price_id FROM accommodation_types WHERE code = $1`
	var t domain.AccommodationType
	var capacity *int
	var priceID *string
	err := r.pool.QueryRow(ctx, q, code).Scan(&t.Code, &t.DisplayName, &capacity, &priceID)
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
	const q = `SELECT code, display_name, capacity, stripe_price_id FROM accommodation_types ORDER BY sort_order, code`
	rows, err := r.pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list accommodation types: %w", err)
	}
	defer rows.Close()

	var out []domain.AccommodationType
	for rows.Next() {
		var t domain.AccommodationType
		var capacity *int
		var priceID *string
		if err := rows.Scan(&t.Code, &t.DisplayName, &capacity, &priceID); err != nil {
			return nil, err
		}
		t.Capacity = capacity
		t.StripePriceID = priceID
		out = append(out, t)
	}
	return out, rows.Err()
}

// ListAccommodationUnits returns all physical units for admin allocation.
func (r *Repository) ListAccommodationUnits(ctx context.Context) ([]domain.AccommodationUnit, error) {
	const q = `SELECT code, accommodation_code, display_name, capacity, sort_order
		  FROM accommodation_units
		 ORDER BY accommodation_code, sort_order, code`
	rows, err := r.pool.Query(ctx, q)
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
	err := r.pool.QueryRow(ctx, q, code).Scan(
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
	tag, err := r.pool.Exec(ctx,
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

// ExtendInvoiceDueAt pushes the due date (admin action).
func (r *Repository) ExtendInvoiceDueAt(ctx context.Context, groupID string, dueAt time.Time) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE registration_groups SET invoice_due_at = $1 WHERE id = $2 AND billing_status = 'invoiced'`,
		dueAt.UTC(), groupID)
	if err != nil {
		return fmt.Errorf("extend invoice due: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("group %q not found or not invoiced", groupID)
	}
	return nil
}
