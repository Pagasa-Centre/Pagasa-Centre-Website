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

func (NoopMailer) SendAllocationReleased(_ context.Context, p AllocationReleased) error {
	subject, _, err := renderAllocationReleased(p)
	if err != nil {
		return err
	}
	log.Printf("[email noop] to=%s subject=%q (allocation released)", p.ToEmail, subject)
	return nil
}

func (NoopMailer) SendWhiteTeamNotification(_ context.Context, p WhiteTeamNotification) error {
	log.Printf("[email noop] to=%s subject=%q (white team)", p.ToEmail, p.Subject)
	return nil
}

func (NoopMailer) SendBalanceInvoice(_ context.Context, p BalanceInvoice) error {
	subject, _, err := renderBalanceInvoice(p)
	if err != nil {
		return err
	}
	log.Printf("[email noop] to=%s subject=%q pay_url=%s (balance invoice)",
		p.ToEmail, subject, p.PayURL)
	return nil
}

func (NoopMailer) SendCoachInvoice(_ context.Context, p CoachInvoice) error {
	subject, _, err := renderCoachInvoice(p)
	if err != nil {
		return err
	}
	log.Printf("[email noop] to=%s subject=%q pay_url=%s (coach invoice)",
		p.ToEmail, subject, p.PayURL)
	return nil
}

func (NoopMailer) SendBalancePaid(_ context.Context, p BalancePaid) error {
	subject, _, err := renderBalancePaid(p)
	if err != nil {
		return err
	}
	log.Printf("[email noop] to=%s subject=%q amount=%s (balance paid)",
		p.ToEmail, subject, p.AmountLabel)
	return nil
}

func (NoopMailer) SendBalancePaidConfirmation(_ context.Context, p BalancePaidConfirmation) error {
	subject, _, err := renderBalancePaidConfirmation(p)
	if err != nil {
		return err
	}
	log.Printf("[email noop] to=%s subject=%q amount=%s (balance paid confirmation)",
		p.ToEmail, subject, p.AmountLabel)
	return nil
}

func (NoopMailer) SendSponsorshipConfirmed(_ context.Context, p SponsorshipConfirmed) error {
	subject, _, err := renderSponsorshipConfirmed(p)
	if err != nil {
		return err
	}
	log.Printf("[email noop] to=%s subject=%q (sponsorship confirmed)", p.ToEmail, subject)
	return nil
}

func (NoopMailer) SendAccommodationChanged(_ context.Context, p AccommodationChangedNotice) error {
	subject, _, err := renderAccommodationChanged(p)
	if err != nil {
		return err
	}
	log.Printf("[email noop] to=%s subject=%q changes=%d (accommodation changed)",
		p.ToEmail, subject, len(p.Items))
	return nil
}
