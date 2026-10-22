package sheetsync

import (
	"context"

	"pagasacentre/backend/internal/registration/domain"
	"pagasacentre/backend/internal/sheets"
)

type groupReader interface {
	FindGroupByID(ctx context.Context, id string) (*domain.Group, error)
	CampersForGroup(ctx context.Context, id string) ([]domain.Camper, error)
	ListWithBilling(ctx context.Context, f domain.ListFilterBilling) ([]domain.Group, error)
}

type sheetRepo interface {
	StampOnly(ctx context.Context, groupID string, meta domain.ActionMeta, tr domain.BillingTransition) error
}

type Service struct {
	reads  groupReader
	repo   sheetRepo
	sheets sheets.Sync
}

func NewService(reads groupReader, repo sheetRepo, sheetSync sheets.Sync) *Service {
	if sheetSync == nil {
		sheetSync = sheets.NewNoopSync()
	}
	return &Service{reads: reads, repo: repo, sheets: sheetSync}
}
