package sheetsync

import (
	"context"

	"pagasacentre/backend/internal/notify"
	"pagasacentre/backend/internal/registration/domain"
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
	sheets notify.SheetSync
}

func NewService(reads groupReader, repo sheetRepo, sheetSync notify.SheetSync) *Service {
	if sheetSync == nil {
		sheetSync = notify.NewNoopSheetSync()
	}
	return &Service{reads: reads, repo: repo, sheets: sheetSync}
}
