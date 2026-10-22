package registration

import (
	"context"
	"fmt"

	"pagasacentre/backend/internal/registration/domain"
	"pagasacentre/backend/pkg/commonlibrary/apperr"
)

var allowedPaymentStatuses = map[string]struct{}{
	domain.PaymentPending:         {},
	domain.PaymentPaid:            {},
	domain.PaymentFailed:          {},
	domain.PaymentFailedCapacity:  {},
	domain.PaymentRefunded:        {},
	domain.PaymentCancelled:       {},
}

func (s *Service) FindGroup(ctx context.Context, id string) (*domain.Group, error) {
	return s.repo.FindGroupByID(ctx, id)
}

func (s *Service) CampersForGroup(ctx context.Context, id string) ([]domain.Camper, error) {
	return s.repo.CampersForGroup(ctx, id)
}

func (s *Service) ListGroupsWithCampers(ctx context.Context, f domain.ListFilterBilling) ([]domain.Group, map[string][]domain.Camper, error) {
	groups, err := s.repo.ListWithBilling(ctx, f)
	if err != nil {
		return nil, nil, fmt.Errorf("list groups: %w", err)
	}
	if len(groups) == 0 {
		return groups, map[string][]domain.Camper{}, nil
	}
	ids := make([]string, len(groups))
	for i, g := range groups {
		ids[i] = g.ID
	}
	campers, err := s.repo.CampersForGroups(ctx, ids)
	if err != nil {
		return nil, nil, fmt.Errorf("list campers: %w", err)
	}
	byGroup := map[string][]domain.Camper{}
	for _, c := range campers {
		byGroup[c.GroupID] = append(byGroup[c.GroupID], c)
	}
	return groups, byGroup, nil
}

func (s *Service) SetPaymentStatus(ctx context.Context, groupID, status string) error {
	if _, ok := allowedPaymentStatuses[status]; !ok {
		return apperr.Invalid("invalid payment_status")
	}
	if err := s.repo.MarkStatus(ctx, groupID, status); err != nil {
		return fmt.Errorf("set payment status: %w", err)
	}
	return nil
}

func (s *Service) ListAccommodationTypes(ctx context.Context) ([]domain.AccommodationType, error) {
	return s.repo.ListAccommodationTypes(ctx)
}

func (s *Service) ListAccommodationUnits(ctx context.Context) ([]domain.AccommodationUnit, error) {
	return s.repo.ListAccommodationUnits(ctx)
}

func (s *Service) SetAccommodationAvailability(ctx context.Context, code string, available bool) error {
	if err := s.repo.SetAccommodationAvailableForRegistration(ctx, code, available); err != nil {
		return fmt.Errorf("set accommodation availability: %w", err)
	}
	return nil
}
