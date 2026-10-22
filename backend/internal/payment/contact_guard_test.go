package payment

import (
	"context"
	"errors"
	"testing"

	"pagasacentre/backend/internal/notify"
	"pagasacentre/backend/internal/registration/domain"
	"pagasacentre/backend/pkg/commonlibrary/apperr"
)

type stubRegRepo struct {
	g *domain.Group
}

func (s stubRegRepo) GetGroupBySession(context.Context, string) (*domain.Group, error) {
	return nil, nil
}
func (s stubRegRepo) FindGroupByID(context.Context, string) (*domain.Group, error) {
	if s.g == nil {
		return nil, nil
	}
	cp := *s.g
	return &cp, nil
}
func (s stubRegRepo) CampersForGroup(context.Context, string) ([]domain.Camper, error) {
	return nil, nil
}
func (s stubRegRepo) ApplySettlement(context.Context, string, string, bool, domain.Settlement) error {
	return nil
}
func (s stubRegRepo) UpdateContact(context.Context, string, string, string, string, string) error {
	return nil
}
func (s stubRegRepo) SetPaymentStatus(context.Context, string, domain.PaymentTransition) error {
	return nil
}

func TestUpdateGroupContact_resendGuard(t *testing.T) {
	svc := NewService(nil, stubRegRepo{g: &domain.Group{
		ID: "g1", PaymentStatus: domain.PaymentPending,
	}}, nil, notify.NoopSheetSync{}, "")
	err := svc.UpdateGroupContact(context.Background(), "g1", "A", "B", "a@b.com", "1", true)
	var de *apperr.Error
	if !errors.As(err, &de) || de.Message != "the confirmation email can only be re-sent once the deposit is paid" {
		t.Fatalf("got %v", err)
	}
}
