package groupsql

import (
	"github.com/jackc/pgx/v5"

	"pagasacentre/backend/internal/registration/domain"
)

const GroupSelectCols = `
	id, contact_first_name, contact_last_name, contact_email, contact_phone,
	payment_status, stripe_session_id, stripe_payment_intent_id,
	total_amount_pence, currency, created_at, paid_at,
	stripe_customer_id, stripe_invoice_id, billing_status, invoice_due_at, balance_paid_at,
	version, last_action, last_action_by, last_action_at, is_free,
	coach_included_in_balance, stripe_coach_invoice_id, coach_invoice_due_at, coach_fee_paid_at,
	coach_fee_waived_at, paid_in_full_at_registration`

const CamperSelectCols = `
	id, group_id, is_main_contact, first_name, last_name, gender, age,
	cell_leader_name, is_cell_leader, attendance_type,
	shirt_size, dietary_requirements, needs_coach,
	accommodation_first_choice, accommodation_second_choice, roommate_requests,
	day_pass_days, day_pass_tshirt_option, day_pass_needs_catering,
	allocated_accommodation_code, allocated_unit_code, billed_stripe_price_id,
	deposit_credit_pence, deposit_owed_pence, deposit_paid_pence, created_at`

func ScanGroup(row pgx.Row) (domain.Group, error) {
	var g domain.Group
	err := row.Scan(
		&g.ID, &g.ContactFirstName, &g.ContactLastName, &g.ContactEmail, &g.ContactPhone,
		&g.PaymentStatus, &g.CheckoutSessionRef, &g.DepositPaymentRef,
		&g.TotalAmountPence, &g.Currency, &g.CreatedAt, &g.PaidAt,
		&g.BillingCustomerRef, &g.BalanceInvoiceRef, &g.BillingStatus, &g.InvoiceDueAt, &g.BalancePaidAt,
		&g.Version, &g.LastAction, &g.LastActionBy, &g.LastActionAt, &g.IsFree,
		&g.CoachIncludedInBalance, &g.CoachInvoiceRef, &g.CoachInvoiceDueAt, &g.CoachFeePaidAt,
		&g.CoachFeeWaivedAt, &g.PaidInFullAtRegistration,
	)
	return g, err
}

func ScanCamper(row pgx.Row) (domain.Camper, error) {
	var c domain.Camper
	err := row.Scan(
		&c.ID, &c.GroupID, &c.IsMainContact, &c.FirstName, &c.LastName, &c.Gender, &c.Age,
		&c.CellLeaderName, &c.IsCellLeader, &c.AttendanceType,
		&c.ShirtSize, &c.DietaryRequirements, &c.NeedsCoach,
		&c.AccommodationFirstChoice, &c.AccommodationSecondChoice, &c.RoommateRequests,
		&c.DayPassDays, &c.DayPassTshirtOption, &c.DayPassNeedsCatering,
		&c.AllocatedAccommodationCode, &c.AllocatedUnitCode, &c.BilledPriceRef,
		&c.DepositCreditPence, &c.DepositOwedPence, &c.DepositPaidPence, &c.CreatedAt,
	)
	return c, err
}
