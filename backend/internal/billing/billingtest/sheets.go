package billingtest

import (
	"context"
	"errors"

	"pagasacentre/backend/internal/sheets"
)

// FailingSheets fails RemoveByGroupID for delete/resync error-path tests.
type FailingSheets struct {
	sheets.NoopSync
}

func (FailingSheets) RemoveByGroupID(context.Context, string) error {
	return errors.New("sheet sync failed")
}

// RecordingSheets counts sheet sync calls for integration assertions.
type RecordingSheets struct {
	sheets.NoopSync
	RemoveCalls  int
	AppendCalls  int
	PendingCalls int
	LastGroupID  string
	LastRows     []sheets.Row
}

func (r *RecordingSheets) RemoveByGroupID(_ context.Context, groupID string) error {
	r.RemoveCalls++
	r.LastGroupID = groupID
	return nil
}

func (r *RecordingSheets) AppendPaidAndRemovePending(_ context.Context, groupID string, rows []sheets.Row) error {
	r.AppendCalls++
	r.LastGroupID = groupID
	r.LastRows = rows
	return nil
}

func (r *RecordingSheets) AppendPending(_ context.Context, rows []sheets.Row) error {
	r.PendingCalls++
	if len(rows) > 0 {
		r.LastGroupID = rows[0].GroupID
	}
	r.LastRows = rows
	return nil
}
