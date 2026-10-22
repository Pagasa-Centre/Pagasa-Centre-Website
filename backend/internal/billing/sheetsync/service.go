package sheetsync

import (
	"context"
	"fmt"
	"pagasacentre/backend/internal/notify"

	"pagasacentre/backend/internal/billing"
	"pagasacentre/backend/internal/registration/domain"
	"pagasacentre/backend/pkg/commonlibrary/apperr"
)

func (s *Service) ResyncSheet(ctx context.Context, groupID, actor string, expectedVersion int) error {
	g, err := s.reads.FindGroupByID(ctx, groupID)
	if err != nil {
		return fmt.Errorf("operation: %w", err)
	}
	if g == nil {
		return domain.ErrGroupNotFound
	}
	switch g.PaymentStatus {
	case domain.PaymentPaid, domain.PaymentPending:
	default:
		return apperr.Invalid(
			"only pending or paid registrations can be re-synced to the sheet")
	}

	campers, err := s.reads.CampersForGroup(ctx, groupID)
	if err != nil {
		return fmt.Errorf("operation: %w", err)
	}
	if len(campers) == 0 {
		return apperr.Invalid("registration has no campers")
	}

	rows := notify.RowsForGroup(g, campers)
	if err := s.sheets.RemoveByGroupID(ctx, groupID); err != nil {
		return fmt.Errorf("operation: %w", err)
	}
	switch g.PaymentStatus {
	case domain.PaymentPaid:
		err = s.sheets.AppendPaidAndRemovePending(ctx, groupID, rows)
	case domain.PaymentPending:
		err = s.sheets.AppendPending(ctx, rows)
	}
	if err != nil {
		return fmt.Errorf("operation: %w", err)
	}

	meta := domain.ActionMeta{
		Actor:           actor,
		Action:          "sheet_resynced",
		ExpectedVersion: expectedVersion,
	}
	if err := s.repo.StampOnly(ctx, groupID, meta, domain.BillingTransition{}); err != nil {
		return billing.MapVersionErr(ctx, s.reads, groupID, err)
	}
	return nil
}

func (s *Service) ResyncAllSheets(ctx context.Context, actor string) (int, map[string]error) {
	errs := map[string]error{}
	synced := 0
	for _, status := range []domain.PaymentStatus{domain.PaymentPaid, domain.PaymentPending} {
		groups, err := s.reads.ListWithBilling(ctx, domain.ListFilterBilling{PaymentStatus: status})
		if err != nil {
			errs["_list"] = err
			return synced, errs
		}
		for _, g := range groups {
			if err := s.ResyncSheet(ctx, g.ID, actor, domain.SkipVersionCheck); err != nil {
				errs[g.ID] = err
				continue
			}
			synced++
		}
	}
	return synced, errs
}
