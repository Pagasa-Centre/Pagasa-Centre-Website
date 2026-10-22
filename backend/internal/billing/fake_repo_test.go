package billing

import (
	"context"

	"pagasacentre/backend/internal/registration/domain"
)

// fakeRepo embeds billingRepo so unexpected calls panic; tests override only what they need.
type fakeRepo struct {
	billingRepo
	groups       map[string]*domain.Group
	campers      map[string][]domain.Camper
	allocateErr  error
	findGroupErr error
}

func (f *fakeRepo) FindGroupByID(_ context.Context, groupID string) (*domain.Group, error) {
	if f.findGroupErr != nil {
		return nil, f.findGroupErr
	}
	if f.groups == nil {
		return nil, nil
	}
	g := f.groups[groupID]
	if g == nil {
		return nil, nil
	}
	cp := *g
	return &cp, nil
}

func (f *fakeRepo) CampersForGroup(_ context.Context, groupID string) ([]domain.Camper, error) {
	if f.campers == nil {
		return nil, nil
	}
	return f.campers[groupID], nil
}

func (f *fakeRepo) AllocateGroup(_ context.Context, _ string, _ []domain.CamperAllocation, _ domain.ActionMeta) error {
	return f.allocateErr
}
