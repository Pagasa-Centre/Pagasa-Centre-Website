package storage

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	commondb "pagasacentre/backend/pkg/commonlibrary/db"

	"pagasacentre/backend/internal/registration/domain"
	"pagasacentre/backend/internal/registration/storage/groupsql"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

const groupSelectCols = groupsql.GroupSelectCols

const camperSelectCols = groupsql.CamperSelectCols

func scanGroup(row pgx.Row) (domain.Group, error) {
	return groupsql.ScanGroup(row)
}

func scanCamper(row pgx.Row) (domain.Camper, error) {
	return groupsql.ScanCamper(row)
}

func (r *Repository) db(ctx context.Context) commondb.Querier {
	return commondb.Conn(ctx, r.pool)
}

// InsertGroup inserts a registration_groups row and returns its UUID.
func (r *Repository) InsertGroup(ctx context.Context, req domain.Submission, totalPence int, currency string, isFree, paidInFull bool) (string, error) {
	const q = `
		INSERT INTO registration_groups (
			contact_first_name, contact_last_name, contact_email, contact_phone,
			total_amount_pence, currency, is_free, paid_in_full_at_registration
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8) RETURNING id`
	var id string
	err := r.db(ctx).QueryRow(ctx, q,
		req.Contact.FirstName, req.Contact.LastName, req.Contact.Email, req.Contact.Phone,
		totalPence, currency, isFree, paidInFull,
	).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("insert group: %w", err)
	}
	return id, nil
}

// InsertCamper inserts a registrations row inside tx.
func (r *Repository) InsertCamper(ctx context.Context, groupID string, c domain.NewCamper) error {
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

	_, err := r.db(ctx).Exec(ctx, q,
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
func (r *Repository) SetStripeSession(ctx context.Context, groupID, sessionID string) error {
	_, err := r.db(ctx).Exec(ctx,
		`UPDATE registration_groups SET stripe_session_id = $1 WHERE id = $2`,
		sessionID, groupID)
	if err != nil {
		return fmt.Errorf("set stripe session: %w", err)
	}
	return nil
}

// GetGroupBySession reads a group by stripe_session_id and locks it FOR UPDATE.
func (r *Repository) GetGroupBySession(ctx context.Context, sessionID string) (*domain.Group, error) {
	const q = `SELECT ` + groupSelectCols + `
		  FROM registration_groups
		 WHERE stripe_session_id = $1
		   FOR UPDATE`
	g, err := scanGroup(r.db(ctx).QueryRow(ctx, q, sessionID))
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
	g, err := scanGroup(r.db(ctx).QueryRow(ctx, q, sessionID))
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
	g, err := scanGroup(r.db(ctx).QueryRow(ctx, q, groupID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find group by id: %w", err)
	}
	return &g, nil
}

// ApplySettlement persists a checkout or zero-total submit with payment CAS and
// optional billing settlement.
func (r *Repository) ApplySettlement(ctx context.Context, groupID, paymentIntentID string, coachIncluded bool, s domain.Settlement) error {
	now := time.Now().UTC()
	set := `UPDATE registration_groups
		    SET payment_status = $1,
		        stripe_payment_intent_id = COALESCE($2, stripe_payment_intent_id),
		        paid_at = $3`
	args := []any{s.Payment.To, nullableString(paymentIntentID), now}
	if s.Billing.ChangesBilling() {
		set += fmt.Sprintf(`, billing_status = $%d, balance_paid_at = $%d`, len(args)+1, len(args)+2)
		args = append(args, s.Billing.To, now)
		set += fmt.Sprintf(`, coach_included_in_balance = coach_included_in_balance OR $%d`, len(args)+1)
		args = append(args, coachIncluded)
	}
	idArg := len(args) + 1
	fromArg := len(args) + 2
	args = append(args, groupID, s.Payment.From)
	where := fmt.Sprintf(` WHERE id = $%d AND payment_status = $%d`, idArg, fromArg)
	if s.Billing.From != "" {
		args = append(args, s.Billing.From)
		where += fmt.Sprintf(` AND billing_status = $%d`, len(args))
	}
	tag, err := r.db(ctx).Exec(ctx, set+where, args...)
	if err != nil {
		return fmt.Errorf("apply settlement: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrVersionConflict
	}
	return nil
}

// SetPaymentStatus updates payment_status with compare-and-set.
func (r *Repository) SetPaymentStatus(ctx context.Context, groupID string, tr domain.PaymentTransition) error {
	tag, err := r.db(ctx).Exec(ctx,
		`UPDATE registration_groups SET payment_status = $1 WHERE id = $2 AND payment_status = $3`,
		tr.To, groupID, tr.From)
	if err != nil {
		return fmt.Errorf("update payment_status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrVersionConflict
	}
	return nil
}

// UpdateContact corrects the group's contact details (name, email, phone).
// Used by the admin dashboard when, e.g., someone mistyped their email at
// registration. Does not touch payment or billing state.
func (r *Repository) UpdateContact(ctx context.Context, groupID, firstName, lastName, email, phone string) error {
	tag, err := r.db(ctx).Exec(ctx,
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
	rows, err := r.db(ctx).Query(ctx, q, groupIDs)
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
