package email

import (
	"context"
	"log"
)

// NoopMailer is used when SMTP isn't configured (local dev). It renders the
// template (so template bugs still surface in dev) and logs a one-line summary,
// then returns nil.
type NoopMailer struct{}

// NewNoopMailer returns a Mailer that logs instead of sending real email.
func NewNoopMailer() *NoopMailer { return &NoopMailer{} }

// SendDepositConfirmation logs that the email would have been sent.
func (NoopMailer) SendDepositConfirmation(_ context.Context, p DepositConfirmation) error {
	subject, _, err := renderDepositConfirmation(p)
	if err != nil {
		// Template breakage should still bubble up in dev so we catch it before
		// shipping. Don't silently swallow.
		return err
	}
	log.Printf("[email noop] to=%s subject=%q amount_pence=%d has_minor=%t",
		p.ToEmail, subject, p.AmountPence, p.HasMinor)
	return nil
}
