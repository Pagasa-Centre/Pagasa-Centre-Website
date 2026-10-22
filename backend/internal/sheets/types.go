package sheets

// Headers is the first-row column labels written to the sheet when a tab is
// empty. Must stay in lock-step with values() in values.go.
var Headers = []any{
	"group_id", "payment_status", "submitted_at", "paid_at",
	"total_amount_pence", "currency",
	"contact_first_name", "contact_last_name", "contact_email", "contact_phone",
	"is_main_contact", "first_name", "last_name", "gender", "age",
	"cell_leader_name", "is_cell_leader", "attendance_type",
	"shirt_size", "dietary_requirements", "needs_coach",
	"accommodation_first_choice", "accommodation_second_choice", "roommate_requests",
	"day_pass_days", "day_pass_tshirt_option", "day_pass_needs_catering",
}
