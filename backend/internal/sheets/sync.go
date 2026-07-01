package sheets

import (
	"context"
	"log"
)

// Sync writes registration rows to the configured Google Sheet. All methods
// are safe to call concurrently. Implementations should never block the
// caller's main flow on a sheets-side failure — log it and move on; the DB
// remains the source of truth.
type Sync interface {
	// AppendPending appends rows to the "Pending" tab. Called immediately
	// after a registration is submitted (regardless of payment outcome).
	AppendPending(ctx context.Context, rows []Row) error

	// AppendPaidAndRemovePending appends rows to the "Paid" tab and deletes
	// any rows in "Pending" whose group_id matches. Called when a registration
	// transitions from pending → paid (via Stripe webhook or zero-total
	// instant-paid path).
	AppendPaidAndRemovePending(ctx context.Context, groupID string, rows []Row) error

	// UpdateContactByGroupID rewrites the contact_* columns for every row
	// (in either tab) whose group_id matches. Called when staff correct a
	// group's contact details (e.g. a mistyped email) after registration.
	UpdateContactByGroupID(ctx context.Context, groupID string, c ContactUpdate) error

	// RemoveByGroupID deletes every row (in Paid and Pending tabs) whose
	// group_id matches. Called when staff permanently delete a registration.
	RemoveByGroupID(ctx context.Context, groupID string) error
}

// ContactUpdate carries the group-level contact fields that can be corrected
// after the fact. Only these four columns are rewritten in the sheet; camper
// rows are otherwise left untouched.
type ContactUpdate struct {
	FirstName string
	LastName  string
	Email     string
	Phone     string
}

// NoopSync is used when no Sheets credentials are configured. It logs each
// call so dev environments can see "would have synced" lines without
// requiring real credentials.
type NoopSync struct{}

// NewNoopSync constructs a NoopSync.
func NewNoopSync() *NoopSync { return &NoopSync{} }

// AppendPending logs the call and returns nil.
func (NoopSync) AppendPending(_ context.Context, rows []Row) error {
	if len(rows) == 0 {
		return nil
	}
	log.Printf("sheets noop: would append %d pending row(s) for group %s", len(rows), rows[0].GroupID)
	return nil
}

// AppendPaidAndRemovePending logs the call and returns nil.
func (NoopSync) AppendPaidAndRemovePending(_ context.Context, groupID string, rows []Row) error {
	log.Printf("sheets noop: would move group %s to paid (%d row(s))", groupID, len(rows))
	return nil
}

// UpdateContactByGroupID logs the call and returns nil.
func (NoopSync) UpdateContactByGroupID(_ context.Context, groupID string, c ContactUpdate) error {
	log.Printf("sheets noop: would update contact for group %s (%s)", groupID, c.Email)
	return nil
}

// RemoveByGroupID logs the call and returns nil.
func (NoopSync) RemoveByGroupID(_ context.Context, groupID string) error {
	log.Printf("sheets noop: would remove rows for group %s", groupID)
	return nil
}
