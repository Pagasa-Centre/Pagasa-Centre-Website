package registration

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

// Pool exposes the underlying pool for callers that need to manage their own
// transactions (e.g. the payment webhook service which spans repositories).
func (r *Repository) Pool() *pgxpool.Pool { return r.pool }

// InsertGroup inserts a registration_groups row inside tx and returns its UUID.
func (r *Repository) InsertGroup(ctx context.Context, tx pgx.Tx, req SubmitRequest, totalPence int, currency string) (string, error) {
	const q = `
		INSERT INTO registration_groups (
			contact_first_name, contact_last_name, contact_email, contact_phone,
			total_amount_pence, currency
		) VALUES ($1,$2,$3,$4,$5,$6) RETURNING id`
	var id string
	err := tx.QueryRow(ctx, q,
		req.Contact.FirstName, req.Contact.LastName, req.Contact.Email, req.Contact.Phone,
		totalPence, currency,
	).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("insert group: %w", err)
	}
	return id, nil
}

// InsertCamper inserts a registrations row inside tx.
func (r *Repository) InsertCamper(ctx context.Context, tx pgx.Tx, groupID string, c CamperDTO) error {
	const q = `
		INSERT INTO registrations (
			group_id, is_main_contact, first_name, last_name, gender, age,
			cell_leader_name, is_cell_leader, attendance_type,
			shirt_size, dietary_requirements, needs_coach, accommodation_code,
			day_pass_days, day_pass_tshirt_option, day_pass_needs_catering
		) VALUES (
			$1,$2,$3,$4,$5,$6,
			$7,$8,$9,
			$10,$11,$12,$13,
			$14,$15,$16
		)`

	var (
		shirtSize, dietary, accommodation, tshirtOption *string
		needsCoach, needsCatering                       *bool
		days                                            []string
	)
	switch c.Attendance.Type {
	case AttendanceFullWeek:
		shirtSize = strPtr(c.Attendance.ShirtSize)
		dietary = strPtr(c.Attendance.DietaryRequirements)
		needsCoach = c.Attendance.NeedsCoach
		accommodation = strPtr(c.Attendance.AccommodationCode)
	case AttendanceDayPass:
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
		shirtSize, dietary, needsCoach, accommodation,
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
func (r *Repository) GetGroupBySession(ctx context.Context, tx pgx.Tx, sessionID string) (*Group, error) {
	const q = `
		SELECT id, contact_first_name, contact_last_name, contact_email, contact_phone,
		       payment_status, stripe_session_id, stripe_payment_intent_id,
		       total_amount_pence, currency, created_at, paid_at
		  FROM registration_groups
		 WHERE stripe_session_id = $1
		   FOR UPDATE`
	var g Group
	err := tx.QueryRow(ctx, q, sessionID).Scan(
		&g.ID, &g.ContactFirstName, &g.ContactLastName, &g.ContactEmail, &g.ContactPhone,
		&g.PaymentStatus, &g.StripeSessionID, &g.StripePaymentIntentID,
		&g.TotalAmountPence, &g.Currency, &g.CreatedAt, &g.PaidAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get group by session: %w", err)
	}
	return &g, nil
}

// AccommodationCountsForGroup returns how many campers in the group requested
// each accommodation code (full-week campers only).
func (r *Repository) AccommodationCountsForGroup(ctx context.Context, tx pgx.Tx, groupID string) (map[string]int, error) {
	const q = `
		SELECT accommodation_code, COUNT(*)
		  FROM registrations
		 WHERE group_id = $1 AND accommodation_code IS NOT NULL
		 GROUP BY accommodation_code`
	rows, err := tx.Query(ctx, q, groupID)
	if err != nil {
		return nil, fmt.Errorf("accommodation counts: %w", err)
	}
	defer rows.Close()

	out := map[string]int{}
	for rows.Next() {
		var code string
		var n int
		if err := rows.Scan(&code, &n); err != nil {
			return nil, err
		}
		out[code] = n
	}
	return out, rows.Err()
}

// MarkPaid transitions the group to 'paid' and stamps paid_at + payment intent.
func (r *Repository) MarkPaid(ctx context.Context, tx pgx.Tx, groupID, paymentIntentID string) error {
	_, err := tx.Exec(ctx,
		`UPDATE registration_groups
		    SET payment_status = 'paid',
		        stripe_payment_intent_id = $1,
		        paid_at = $2
		  WHERE id = $3`,
		paymentIntentID, time.Now().UTC(), groupID)
	if err != nil {
		return fmt.Errorf("mark paid: %w", err)
	}
	return nil
}

// MarkFailedCapacity transitions a group to failed_capacity (race-loser).
func (r *Repository) MarkFailedCapacity(ctx context.Context, tx pgx.Tx, groupID, paymentIntentID string) error {
	_, err := tx.Exec(ctx,
		`UPDATE registration_groups
		    SET payment_status = 'failed_capacity',
		        stripe_payment_intent_id = COALESCE($1, stripe_payment_intent_id)
		  WHERE id = $2`,
		nullableString(paymentIntentID), groupID)
	if err != nil {
		return fmt.Errorf("mark failed_capacity: %w", err)
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

// ListFilter narrows admin listings.
type ListFilter struct {
	PaymentStatus string
}

// List returns groups (newest first), optionally filtered by status.
func (r *Repository) List(ctx context.Context, f ListFilter) ([]Group, error) {
	args := []any{}
	q := `SELECT id, contact_first_name, contact_last_name, contact_email, contact_phone,
	             payment_status, stripe_session_id, stripe_payment_intent_id,
	             total_amount_pence, currency, created_at, paid_at
	        FROM registration_groups`
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

	var out []Group
	for rows.Next() {
		var g Group
		if err := rows.Scan(
			&g.ID, &g.ContactFirstName, &g.ContactLastName, &g.ContactEmail, &g.ContactPhone,
			&g.PaymentStatus, &g.StripeSessionID, &g.StripePaymentIntentID,
			&g.TotalAmountPence, &g.Currency, &g.CreatedAt, &g.PaidAt,
		); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

// CampersForGroup returns the camper rows for a single group.
func (r *Repository) CampersForGroup(ctx context.Context, groupID string) ([]Camper, error) {
	return r.campersByGroupIDs(ctx, []string{groupID})
}

// CampersForGroups returns campers across multiple groups in one query.
func (r *Repository) CampersForGroups(ctx context.Context, groupIDs []string) ([]Camper, error) {
	if len(groupIDs) == 0 {
		return nil, nil
	}
	return r.campersByGroupIDs(ctx, groupIDs)
}

func (r *Repository) campersByGroupIDs(ctx context.Context, groupIDs []string) ([]Camper, error) {
	const q = `
		SELECT id, group_id, is_main_contact, first_name, last_name, gender, age,
		       cell_leader_name, is_cell_leader, attendance_type,
		       shirt_size, dietary_requirements, needs_coach, accommodation_code,
		       day_pass_days, day_pass_tshirt_option, day_pass_needs_catering, created_at
		  FROM registrations
		 WHERE group_id = ANY($1)
		 ORDER BY group_id, is_main_contact DESC, created_at`
	rows, err := r.pool.Query(ctx, q, groupIDs)
	if err != nil {
		return nil, fmt.Errorf("campers by group: %w", err)
	}
	defer rows.Close()

	var out []Camper
	for rows.Next() {
		var c Camper
		if err := rows.Scan(
			&c.ID, &c.GroupID, &c.IsMainContact, &c.FirstName, &c.LastName, &c.Gender, &c.Age,
			&c.CellLeaderName, &c.IsCellLeader, &c.AttendanceType,
			&c.ShirtSize, &c.DietaryRequirements, &c.NeedsCoach, &c.AccommodationCode,
			&c.DayPassDays, &c.DayPassTshirtOption, &c.DayPassNeedsCatering, &c.CreatedAt,
		); err != nil {
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
