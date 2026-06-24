package billing

import (
	"context"
	"errors"
	"fmt"

	commonerrors "pagasacentre/backend/pkg/commonlibrary/errors"
	"pagasacentre/backend/internal/registration/domain"
)

func ExpectedVersion(v *int) int {
	if v == nil {
		return domain.SkipVersionCheck
	}
	return *v
}

func expectedVersion(v *int) int { return ExpectedVersion(v) }

func billingStatusLabel(status string) string {
	switch status {
	case domain.BillingNone:
		return "Needs accommodation"
	case domain.BillingAllocated:
		return "Ready to invoice"
	case domain.BillingInvoiced:
		return "Awaiting payment"
	case domain.BillingBalancePaid:
		return "Paid in full"
	case domain.BillingFreeConfirmed:
		return "Free place confirmed"
	case domain.BillingReleased:
		return "Released"
	default:
		return status
	}
}

type groupReader interface {
	FindGroupByID(ctx context.Context, groupID string) (*domain.Group, error)
}

func staleConflict(ctx context.Context, repo groupReader, groupID string) commonerrors.APIError {
	who := "someone else"
	state := "updated"
	if g, err := repo.FindGroupByID(ctx, groupID); err == nil && g != nil {
		if g.LastActionBy != nil && *g.LastActionBy != "" {
			who = *g.LastActionBy
		}
		state = billingStatusLabel(g.BillingStatus)
	}
	msg := fmt.Sprintf(
		"This group was just updated by %s (%s). Your view was refreshed — please check and try again.",
		who, state,
	)
	return commonerrors.Conflict("stale_state", msg, nil)
}

func mapVersionErr(ctx context.Context, repo groupReader, groupID string, err error) error {
	if errors.Is(err, domain.ErrVersionConflict) {
		return staleConflict(ctx, repo, groupID)
	}
	return commonerrors.Internal(err.Error())
}
