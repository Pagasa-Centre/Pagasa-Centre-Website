package payment

import (
	"context"
	"testing"

	"pagasacentre/backend/internal/registration/domain"
)

type fakeTx struct {
	lastErr error
}

func (f *fakeTx) WithTx(_ context.Context, fn func(context.Context) error) error {
	f.lastErr = fn(context.Background())
	return f.lastErr
}

type expiredRegRepo struct {
	regRepo
	group           *domain.Group
	setPaymentCalls int
}

func (f *expiredRegRepo) GetGroupBySession(_ context.Context, _ string) (*domain.Group, error) {
	return f.group, nil
}

func (f *expiredRegRepo) SetPaymentStatus(_ context.Context, _ string, _ domain.PaymentTransition) error {
	f.setPaymentCalls++
	return nil
}

func TestHandleCheckoutExpired_NonPendingSkipsMarkStatus(t *testing.T) {
	tx := &fakeTx{}
	repo := &expiredRegRepo{
		group: &domain.Group{
			ID:            "g1",
			PaymentStatus: domain.PaymentPaid,
		},
	}
	svc := NewService(tx, repo, nil, nil, "")
	if err := svc.HandleCheckoutExpired(context.Background(), "sess_paid"); err != nil {
		t.Fatalf("HandleCheckoutExpired: %v", err)
	}
	if repo.setPaymentCalls != 0 {
		t.Fatalf("SetPaymentStatus called %d times, want 0", repo.setPaymentCalls)
	}
}
