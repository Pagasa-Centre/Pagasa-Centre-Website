package registration

import (
	"context"
	"errors"
	"fmt"

	"pagasacentre/backend/internal/registration/domain"
	"pagasacentre/backend/pkg/commonlibrary/apperr"
)

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
	to, ok := domain.ParsePaymentStatus(status)
	if !ok {
		return apperr.Invalid("invalid payment_status")
	}
	g, err := s.repo.FindGroupByID(ctx, groupID)
	if err != nil {
		return fmt.Errorf("set payment status: %w", err)
	}
	if g == nil {
		return domain.ErrGroupNotFound
	}
	tr := g.OverridePaymentStatus(to)
	if err := s.repo.SetPaymentStatus(ctx, groupID, tr); err != nil {
		if errors.Is(err, domain.ErrVersionConflict) {
			return apperr.Conflict("stale_state", "This registration was just updated. Refresh and try again.")
		}
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
