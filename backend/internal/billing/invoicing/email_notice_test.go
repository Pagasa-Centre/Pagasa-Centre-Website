package invoicing

import (
	"context"
	"testing"

	"pagasacentre/backend/internal/billing"
	"pagasacentre/backend/internal/email"
)

type recordingMailer struct {
	email.NoopMailer
	accomChanged []email.AccommodationChangedNotice
}

func (m *recordingMailer) SendAccommodationChanged(_ context.Context, p email.AccommodationChangedNotice) error {
	m.accomChanged = append(m.accomChanged, p)
	return nil
}

func TestSendAccommodationChangedNotice(t *testing.T) {
	mailer := &recordingMailer{}
	svc := NewService(nil, nil, nil, mailer, nil, billing.NewPricer(nil, billing.Config{}, nil), billing.Config{})

	changes := []email.AccommodationChange{{
		CamperName:   "Alex Test",
		FirstChoice:  "Lodge",
		SecondChoice: "Cabin",
		Allocated:    "Tent",
		TentGuidance: true,
	}}
	svc.sendAccommodationChangedNotice(context.Background(), "family@example.com", "Sam", changes, true)

	if len(mailer.accomChanged) != 1 {
		t.Fatalf("expected 1 accommodation-changed email, got %d", len(mailer.accomChanged))
	}
	if !mailer.accomChanged[0].AwaitingPayment {
		t.Fatal("expected AwaitingPayment true")
	}
	if mailer.accomChanged[0].ToEmail != "family@example.com" {
		t.Fatalf("to = %q", mailer.accomChanged[0].ToEmail)
	}

	before := len(mailer.accomChanged)
	svc.sendAccommodationChangedNotice(context.Background(), "family@example.com", "Sam", nil, true)
	if len(mailer.accomChanged) != before {
		t.Fatal("should not send when changes empty")
	}
}
