package storage

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"pagasacentre/backend/internal/registration/domain"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

const groupSelectCols = `
	id, contact_first_name, contact_last_name, contact_email, contact_phone,
	payment_status, stripe_session_id, stripe_payment_intent_id,
	total_amount_pence, currency, created_at, paid_at,
	stripe_customer_id, stripe_invoice_id, billing_status, invoice_due_at, balance_paid_at,
	version, last_action, last_action_by, last_action_at, is_free,
	coach_included_in_balance, stripe_coach_invoice_id, coach_invoice_due_at, coach_fee_paid_at,
	coach_fee_waived_at, paid_in_full_at_registration`

func scanGroup(row pgx.Row) (domain.Group, error) {
	var g domain.Group
	err := row.Scan(
		&g.ID, &g.ContactFirstName, &g.ContactLastName, &g.ContactEmail, &g.ContactPhone,
		&g.PaymentStatus, &g.StripeSessionID, &g.StripePaymentIntentID,
		&g.TotalAmountPence, &g.Currency, &g.CreatedAt, &g.PaidAt,
		&g.StripeCustomerID, &g.StripeInvoiceID, &g.BillingStatus, &g.InvoiceDueAt, &g.BalancePaidAt,
		&g.Version, &g.LastAction, &g.LastActionBy, &g.LastActionAt, &g.IsFree,
		&g.CoachIncludedInBalance, &g.StripeCoachInvoiceID, &g.CoachInvoiceDueAt, &g.CoachFeePaidAt,
		&g.CoachFeeWaivedAt, &g.PaidInFullAtRegistration,
	)
	return g, err
}

const camperSelectCols = `
	id, group_id, is_main_contact, first_name, last_name, gender, age,
	cell_leader_name, is_cell_leader, attendance_type,
	shirt_size, dietary_requirements, needs_coach,
	accommodation_first_choice, accommodation_second_choice, roommate_requests,
	day_pass_days, day_pass_tshirt_option, day_pass_needs_catering,
	allocated_accommodation_code, allocated_unit_code, billed_stripe_price_id,
	deposit_credit_pence, created_at`

func scanCamper(row pgx.Row) (domain.Camper, error) {
	var c domain.Camper
	err := row.Scan(
		&c.ID, &c.GroupID, &c.IsMainContact, &c.FirstName, &c.LastName, &c.Gender, &c.Age,
		&c.CellLeaderName, &c.IsCellLeader, &c.AttendanceType,
		&c.ShirtSize, &c.DietaryRequirements, &c.NeedsCoach,
		&c.AccommodationFirstChoice, &c.AccommodationSecondChoice, &c.RoommateRequests,
		&c.DayPassDays, &c.DayPassTshirtOption, &c.DayPassNeedsCatering,
		&c.AllocatedAccommodationCode, &c.AllocatedUnitCode, &c.BilledStripePriceID,
		&c.DepositCreditPence, &c.CreatedAt,
	)
	return c, err
}

// Pool exposes the underlying pool for callers that need to manage their own
// transactions (e.g. the payment webhook service which spans repositories).
func (r *Repository) Pool() *pgxpool.Pool { return r.pool }

// InsertGroup inserts a registration_groups row inside tx and returns its UUID.
func (r *Repository) InsertGroup(ctx context.Context, tx pgx.Tx, req domain.SubmitRequest, totalPence int, currency string, isFree, paidInFull bool) (string, error) {
	const q = `
		INSERT INTO registration_groups (
			contact_first_name, contact_last_name, contact_email, contact_phone,
			total_amount_pence, currency, is_free, paid_in_full_at_registration
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8) RETURNING id`
	var id string
	err := tx.QueryRow(ctx, q,
		req.Contact.FirstName, req.Contact.LastName, req.Contact.Email, req.Contact.Phone,
		totalPence, currency, isFree, paidInFull,
	).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("insert group: %w", err)
	}
	return id, nil
}

// InsertCamper inserts a registrations row inside tx.
func (r *Repository) InsertCamper(ctx context.Context, tx pgx.Tx, groupID string, c domain.CamperDTO) error {
	const q = `
		INSERT INTO registrations (
			group_id, is_main_contact, first_name, last_name, gender, age,
			cell_leader_name, is_cell_leader, attendance_type,
			shirt_size, dietary_requirements, needs_coach,
			accommodation_first_choice, accommodation_second_choice, roommate_requests,
			day_pass_days, day_pass_tshirt_option, day_pass_needs_catering
		) VALUES (
			$1,$2,$3,$4,$5,$6,
			$7,$8,$9,
			$10,$11,$12,
			$13,$14,$15,
			$16,$17,$18
		)`

	var (
		shirtSize, dietary, firstChoice, secondChoice, roommate, tshirtOption *string
		needsCoach, needsCatering                                             *bool
		days                                                                  []string
	)
	switch c.Attendance.Type {
	case domain.AttendanceFullWeek:
		shirtSize = strPtr(c.Attendance.ShirtSize)
		dietary = strPtr(c.Attendance.DietaryRequirements)
		needsCoach = c.Attendance.NeedsCoach
		firstChoice = strPtr(c.Attendance.AccommodationFirstChoice)
		secondChoice = strPtr(c.Attendance.AccommodationSecondChoice)
		roommate = strPtr(c.Attendance.RoommateRequests)
	case domain.AttendanceDayPass:
		dietary = strPtr(c.Attendance.DietaryRequirements)
		if s := c.Attendance.ShirtSize; s != "" {
			shirtSize = &s
		}
		days = c.Attendance.Days
		o := c.Attendance.TshirtOption
		tshirtOption = &o
		needsCatering = c.Attendance.NeedsCatering
	}

	_, err := tx.Exec(ctx, q,
		groupID, c.IsMainContact, c.FirstName, c.LastName, c.Gender, c.Age,
		c.CellLeaderName, c.IsCellLeader, c.Attendance.Type,
		shirtSize, dietary, needsCoach,
		firstChoice, secondChoice, roommate,
		days, tshirtOption, needsCatering,
	)
	if err != nil {
		return fmt.Errorf("insert camper: %w", err)
	}
	return nil
}

// SetStripeSession records the Stripe Checkout session id on the group.
func (r *Repository) SetStripeSession(ctx context.Context, tx pgx.Tx, groupID, sessionID string) error {
	_, err := tx.Exec(ctx,
		`UPDATE registration_groups SET stripe_session_id = $1 WHERE id = $2`,
		sessionID, groupID)
	if err != nil {
		return fmt.Errorf("set stripe session: %w", err)
	}
	return nil
}

// GetGroupBySession reads a group by stripe_session_id and locks it FOR UPDATE.
func (r *Repository) GetGroupBySession(ctx context.Context, tx pgx.Tx, sessionID string) (*domain.Group, error) {
	const q = `SELECT ` + groupSelectCols + `
		  FROM registration_groups
		 WHERE stripe_session_id = $1
		   FOR UPDATE`
	g, err := scanGroup(tx.QueryRow(ctx, q, sessionID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get group by session: %w", err)
	}
	return &g, nil
}

// FindGroupBySessionID is a non-locking read of a group by stripe_session_id.
// Used by the success-page summary endpoint, which only needs read access.
// Returns (nil, nil) if no group matches.
func (r *Repository) FindGroupBySessionID(ctx context.Context, sessionID string) (*domain.Group, error) {
	const q = `SELECT ` + groupSelectCols + `
		  FROM registration_groups
		 WHERE stripe_session_id = $1`
	g, err := scanGroup(r.pool.QueryRow(ctx, q, sessionID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find group by session: %w", err)
	}
	return &g, nil
}

// FindGroupByID is a non-locking read of a group by primary key. Used by the
// success-page summary endpoint for the £0 / day-pass-only path which has no
// Stripe session ID. Returns (nil, nil) if no group matches.
func (r *Repository) FindGroupByID(ctx context.Context, groupID string) (*domain.Group, error) {
	const q = `SELECT ` + groupSelectCols + `
		  FROM registration_groups
		 WHERE id = $1`
	g, err := scanGroup(r.pool.QueryRow(ctx, q, groupID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find group by id: %w", err)
	}
	return &g, nil
}

// MarkPaid transitions the group to 'paid' and stamps paid_at + payment intent.
// paymentIntentID may be empty (used by the £0 day-pass-only Submit path where
// no Stripe session was ever created).
func (r *Repository) MarkPaid(ctx context.Context, tx pgx.Tx, groupID, paymentIntentID string) error {
	_, err := tx.Exec(ctx,
		`UPDATE registration_groups
		    SET payment_status = 'paid',
		        stripe_payment_intent_id = COALESCE($1, stripe_payment_intent_id),
		        paid_at = $2
		  WHERE id = $3`,
		nullableString(paymentIntentID), time.Now().UTC(), groupID)
	if err != nil {
		return fmt.Errorf("mark paid: %w", err)
	}
	return nil
}

// MarkPaidInFull transitions a prepaid registration group to fully settled:
// payment paid plus billing balance_paid.
func (r *Repository) MarkPaidInFull(ctx context.Context, tx pgx.Tx, groupID, paymentIntentID string, coachIncluded bool) error {
	_, err := tx.Exec(ctx,
		`UPDATE registration_groups
		    SET payment_status = 'paid',
		        stripe_payment_intent_id = COALESCE($1, stripe_payment_intent_id),
		        paid_at = $2,
		        billing_status = 'balance_paid',
		        balance_paid_at = $2,
		        coach_included_in_balance = coach_included_in_balance OR $3
		  WHERE id = $4`,
		nullableString(paymentIntentID), time.Now().UTC(), coachIncluded, groupID)
	if err != nil {
		return fmt.Errorf("mark paid in full: %w", err)
	}
	return nil
}

// MarkStatus changes payment_status (used by admin + cancelled webhook).
func (r *Repository) MarkStatus(ctx context.Context, groupID, status string) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE registration_groups SET payment_status = $1 WHERE id = $2`,
		status, groupID)
	if err != nil {
		return fmt.Errorf("update payment_status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("group %q not found", groupID)
	}
	return nil
}

// UpdateContact corrects the group's contact details (name, email, phone).
// Used by the admin dashboard when, e.g., someone mistyped their email at
// registration. Does not touch payment or billing state.
func (r *Repository) UpdateContact(ctx context.Context, groupID, firstName, lastName, email, phone string) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE registration_groups
		    SET contact_first_name = $1,
		        contact_last_name  = $2,
		        contact_email      = $3,
		        contact_phone      = $4
		  WHERE id = $5`,
		firstName, lastName, email, phone, groupID)
	if err != nil {
		return fmt.Errorf("update contact: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("group %q not found", groupID)
	}
	return nil
}

// MarkStatusInTx is MarkStatus but inside an existing transaction.
func (r *Repository) MarkStatusInTx(ctx context.Context, tx pgx.Tx, groupID, status string) error {
	_, err := tx.Exec(ctx,
		`UPDATE registration_groups SET payment_status = $1 WHERE id = $2`,
		status, groupID)
	if err != nil {
		return fmt.Errorf("update payment_status: %w", err)
	}
	return nil
}

// List returns groups (newest first), optionally filtered by status.
func (r *Repository) List(ctx context.Context, f domain.ListFilter) ([]domain.Group, error) {
	args := []any{}
	q := `SELECT ` + groupSelectCols + ` FROM registration_groups`
	if f.PaymentStatus != "" {
		q += ` WHERE payment_status = $1`
		args = append(args, f.PaymentStatus)
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

// CampersForGroup returns the camper rows for a single group.
func (r *Repository) CampersForGroup(ctx context.Context, groupID string) ([]domain.Camper, error) {
	return r.campersByGroupIDs(ctx, []string{groupID})
}

// CampersForGroups returns campers across multiple groups in one query.
func (r *Repository) CampersForGroups(ctx context.Context, groupIDs []string) ([]domain.Camper, error) {
	if len(groupIDs) == 0 {
		return nil, nil
	}
	return r.campersByGroupIDs(ctx, groupIDs)
}

func (r *Repository) campersByGroupIDs(ctx context.Context, groupIDs []string) ([]domain.Camper, error) {
	const q = `SELECT ` + camperSelectCols + `
		  FROM registrations
		 WHERE group_id = ANY($1)
		 ORDER BY group_id, is_main_contact DESC, created_at`
	rows, err := r.pool.Query(ctx, q, groupIDs)
	if err != nil {
		return nil, fmt.Errorf("campers by group: %w", err)
	}
	defer rows.Close()

	var out []domain.Camper
	for rows.Next() {
		c, err := scanCamper(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func nullableString(s string) any {
	if s == "" {
		return nil
	}
	return s
}
