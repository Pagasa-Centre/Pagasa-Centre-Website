package billingtest

import (
	"context"

	"pagasacentre/backend/internal/email"
)

type RecordingMailer struct {
	email.NoopMailer
	Sponsorships   []email.SponsorshipConfirmed
	BalancePaid    []email.BalancePaidConfirmation
	AccomChanged   []email.AccommodationChangedNotice
	BookingUpdated []email.BookingUpdated
}

func (m *RecordingMailer) SendBookingUpdated(_ context.Context, p email.BookingUpdated) error {
	m.BookingUpdated = append(m.BookingUpdated, p)
	return nil
}

func (m *RecordingMailer) SendSponsorshipConfirmed(_ context.Context, p email.SponsorshipConfirmed) error {
	m.Sponsorships = append(m.Sponsorships, p)
	return nil
}

func (m *RecordingMailer) SendBalancePaidConfirmation(_ context.Context, p email.BalancePaidConfirmation) error {
	m.BalancePaid = append(m.BalancePaid, p)
	return nil
}

func (m *RecordingMailer) SendAccommodationChanged(_ context.Context, p email.AccommodationChangedNotice) error {
	m.AccomChanged = append(m.AccomChanged, p)
	return nil
}
