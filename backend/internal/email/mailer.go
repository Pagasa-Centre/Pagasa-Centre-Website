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
	SendBalanceInvoice(ctx context.Context, p BalanceInvoice) error
	SendBalancePaid(ctx context.Context, p BalancePaid) error
	SendBalancePaidConfirmation(ctx context.Context, p BalancePaidConfirmation) error
	SendSponsorshipConfirmed(ctx context.Context, p SponsorshipConfirmed) error
	SendAccommodationChanged(ctx context.Context, p AccommodationChangedNotice) error
}

// AccommodationChange describes one camper placed outside their 1st/2nd choice.
type AccommodationChange struct {
	CamperName   string
	FirstChoice  string // display name; empty if none recorded
	SecondChoice string // display name; empty if none recorded
	Allocated    string // display name of tier actually assigned
	TentGuidance bool   // true when allocated tier is tent — bring your own
}

// AccommodationChangedNotice is the dedicated heads-up email sent when a
// family is invoiced for accommodation that is neither their 1st nor 2nd choice.
type AccommodationChangedNotice struct {
	ToEmail         string
	ToName          string
	Items           []AccommodationChange
	AwaitingPayment bool // true when a balance invoice is on its way / open
}

// BalancePaidConfirmation tells the family their balance is paid and their
// camp place is fully confirmed. Distinct from BalancePaid, which is the
// internal White Team notification.
type BalancePaidConfirmation struct {
	ToEmail     string
	ToName      string
	AmountLabel string // optional, e.g. "£250.00" (blank if unknown)
	// Items is one line per full-week camper, e.g. "Josh Basco — Lodge".
	Items []string
	// Changes lists campers placed outside their preferences (inline callout).
	Changes []AccommodationChange
}

// SponsorshipConfirmed tells a church-sponsored family that their accommodation
// is allocated and their place is fully confirmed — nothing to pay.
type SponsorshipConfirmed struct {
	ToEmail string
	ToName  string
	// Items is one line per full-week camper, e.g. "Josh Basco — Lodge".
	Items []string
	// Changes lists campers placed outside their preferences (inline callout).
	Changes []AccommodationChange
}

// BalancePaid is the notification (to the White Team) that a group has paid
// their balance invoice in full.
type BalancePaid struct {
	ToEmail      string // recipient (White Team ops inbox)
	ContactName  string // who the payment was for
	ContactEmail string
	AmountLabel  string // e.g. "£200.00" (blank if unknown)
	PaidDate     string // pre-formatted, e.g. "4 Jun 2026"
	// Items is one line per person covered, e.g. "Josh Basco — Lodge".
	Items []string
}

// BalanceInvoice emails the family their Stripe balance-invoice payment link.
// We email the link ourselves (rather than relying on Stripe to send it) so it
// works regardless of the Stripe account's invoice-email capability.
type BalanceInvoice struct {
	ToEmail     string
	ToName      string
	PayURL      string // Stripe hosted invoice URL
	DueDate     string // pre-formatted, e.g. "19 Jun 2026"
	AmountLabel string // optional, e.g. "£250.00" (blank if unknown)
	// Items is one line per thing being paid for, e.g.
	// "Josh Basco — Lodge". Shown as a bullet list in the email.
	Items []string
	// Changes lists campers placed outside their preferences (inline callout).
	Changes []AccommodationChange
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
	IsFree         bool   // church-sponsored registration (not day-pass £0)
}
