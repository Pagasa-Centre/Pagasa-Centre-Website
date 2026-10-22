package billingtest

import (
	"context"
	"errors"

	"pagasacentre/backend/internal/notify"
)

// FailingSheets fails RemoveByGroupID for delete/resync error-path tests.
type FailingSheets struct {
	notify.NoopSheetSync
}

func (FailingSheets) RemoveByGroupID(context.Context, string) error {
	return errors.New("sheet sync failed")
}

// RecordingSheets counts sheet sync calls for integration assertions.
type RecordingSheets struct {
	notify.NoopSheetSync
	RemoveCalls  int
	AppendCalls  int
	PendingCalls int
	LastGroupID  string
	LastRows     []notify.SheetRow
}

func (r *RecordingSheets) RemoveByGroupID(_ context.Context, groupID string) error {
	r.RemoveCalls++
	r.LastGroupID = groupID
	return nil
}

func (r *RecordingSheets) AppendPaidAndRemovePending(_ context.Context, groupID string, rows []notify.SheetRow) error {
	r.AppendCalls++
	r.LastGroupID = groupID
	r.LastRows = rows
	return nil
}

func (r *RecordingSheets) AppendPending(_ context.Context, rows []notify.SheetRow) error {
	r.PendingCalls++
	if len(rows) > 0 {
		r.LastGroupID = rows[0].GroupID
	}
	r.LastRows = rows
	return nil
}
