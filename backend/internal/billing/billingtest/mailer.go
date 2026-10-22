package billingtest

import (
	"context"

	"pagasacentre/backend/internal/notify"
)

type RecordingMailer struct {
	notify.NopMailer
	Sponsorships   []notify.SponsorshipConfirmed
	BalancePaid    []notify.BalancePaidConfirmation
	AccomChanged   []notify.AccommodationChangedNotice
	BookingUpdated []notify.BookingUpdated
}

func (m *RecordingMailer) SendBookingUpdated(_ context.Context, p notify.BookingUpdated) error {
	m.BookingUpdated = append(m.BookingUpdated, p)
	return nil
}

func (m *RecordingMailer) SendSponsorshipConfirmed(_ context.Context, p notify.SponsorshipConfirmed) error {
	m.Sponsorships = append(m.Sponsorships, p)
	return nil
}

func (m *RecordingMailer) SendBalancePaidConfirmation(_ context.Context, p notify.BalancePaidConfirmation) error {
	m.BalancePaid = append(m.BalancePaid, p)
	return nil
}

func (m *RecordingMailer) SendAccommodationChanged(_ context.Context, p notify.AccommodationChangedNotice) error {
	m.AccomChanged = append(m.AccomChanged, p)
	return nil
}
