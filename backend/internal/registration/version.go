package registration

import "errors"

// ErrVersionConflict is returned when an optimistic version check fails.
var ErrVersionConflict = errors.New("version conflict")

// SkipVersionCheck tells repo mutators to skip the version predicate (bulk/cron).
const SkipVersionCheck = -1

// ActionMeta carries attribution and optional optimistic concurrency for a
// group-level mutation initiated from the admin dashboard (or "system"/"Stripe").
type ActionMeta struct {
	Actor           string
	Action          string
	ExpectedVersion int // use SkipVersionCheck to bypass
}

func (m ActionMeta) enforceVersion() bool {
	return m.ExpectedVersion >= 0
}
