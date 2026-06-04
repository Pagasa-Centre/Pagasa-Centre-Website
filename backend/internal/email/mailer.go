// Package email sends transactional emails for the camp registration flow.
//
// The package exposes a single Mailer interface that the registration and
// payment services depend on. The interface only has one method today
// (SendDepositConfirmation); future emails (e.g. accommodation confirmation
// once final payment is in) will live alongside it as new methods on the same
// interface, keeping the dependency graph flat.
package email

import "context"

// Mailer is the surface area the rest of the codebase depends on. Implemented
// by SMTPMailer (production) and NoopMailer (local dev without SMTP creds).
type Mailer interface {
	SendDepositConfirmation(ctx context.Context, p DepositConfirmation) error
	SendAllocationReleased(ctx context.Context, p AllocationReleased) error
	SendWhiteTeamNotification(ctx context.Context, p WhiteTeamNotification) error
}

// AllocationReleased notifies a family their placement was released (unpaid).
type AllocationReleased struct {
	ToEmail     string
	ToName      string
	CamperNames []string
	Reason      string
}

// WhiteTeamNotification is an internal ops email to WHITE_TEAM_EMAIL.
type WhiteTeamNotification struct {
	ToEmail string
	Subject string
	Body    string // plain text, wrapped in minimal HTML
}

// DepositConfirmation is the data the deposit-received email is built from.
//
// AmountPence == 0 signals a day-pass-only registration: the template adjusts
// the subject and body to omit deposit-specific wording in that case.
type DepositConfirmation struct {
	ToEmail        string
	ToName         string
	AmountPence    int
	Currency       string
	CamperCount    int // full-week + day-pass total
	HasMinor       bool
	ConsentFormURL string // empty if HasMinor is false
}
