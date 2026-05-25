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
