package registration

import (
	"context"
	"errors"
	"testing"

	"pagasacentre/backend/internal/registration/domain"
)

type recordingTx struct {
	fnErr error
}

func (r *recordingTx) WithTx(_ context.Context, fn func(context.Context) error) error {
	r.fnErr = fn(context.Background())
	return r.fnErr
}

type submitCamp struct{}

func (submitCamp) RegistrationsOpen(context.Context) (bool, error) { return true, nil }
func (submitCamp) RegistrationPaymentMode(context.Context) (string, error) {
	return domain.PaymentModeDeposit, nil
}

type submitPrices struct{}

func (submitPrices) GetPrice(context.Context, string) (PriceRow, error) {
	return PriceRow{AmountPence: 5000, Currency: "GBP"}, nil
}

type failStripe struct{}

func (failStripe) CreateCheckoutSession(context.Context, CheckoutParams) (CheckoutSession, error) {
	return CheckoutSession{}, errors.New("stripe unavailable")
}

type submitRepo struct {
	groupRepo
	inserted bool
}

func (s *submitRepo) AccommodationAvailability(context.Context) (map[string]bool, error) {
	return map[string]bool{"lodge": true, "cabin": true}, nil
}

func (s *submitRepo) InsertGroup(_ context.Context, _ domain.Submission, _ int, _ string, _, _ bool) (string, error) {
	s.inserted = true
	return "g-new", nil
}

func (s *submitRepo) InsertCamper(context.Context, string, domain.NewCamper) error { return nil }

func TestSubmit_StripeFailureRollsBackViaWithTx(t *testing.T) {
	tx := &recordingTx{}
	repo := &submitRepo{}
	svc := NewService(repo, tx, submitPrices{}, failStripe{}, nil, submitCamp{}, nil, nil, "", Config{})
	req := domain.Submission{
		Contact: domain.Contact{
			FirstName: "Test", LastName: "User", Email: "t@example.com", Phone: "+44",
		},
		Campers: []domain.NewCamper{{
			FirstName: "Test", LastName: "User", Gender: "female", Age: 25,
			CellLeaderName: "Leader", IsMainContact: true,
			Attendance: domain.Attendance{
				Type:                      domain.AttendanceFullWeek,
				ShirtSize:                 "adult_m",
				AccommodationFirstChoice:  "lodge",
				AccommodationSecondChoice: "cabin",
			},
		}},
	}
	_, err := svc.Submit(context.Background(), req)
	if err == nil {
		t.Fatal("expected stripe error")
	}
	if tx.fnErr == nil {
		t.Fatal("WithTx did not record closure error")
	}
	if !repo.inserted {
		t.Fatal("expected InsertGroup to run before stripe failure")
	}
}
