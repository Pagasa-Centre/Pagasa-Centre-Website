package billing

import (
	"context"
	"errors"
	"fmt"

	"pagasacentre/backend/internal/httpx"
	"pagasacentre/backend/internal/registration"
)

func expectedVersion(v *int) int {
	if v == nil {
		return registration.SkipVersionCheck
	}
	return *v
}

func billingStatusLabel(status string) string {
	switch status {
	case registration.BillingNone:
		return "Needs accommodation"
	case registration.BillingAllocated:
		return "Ready to invoice"
	case registration.BillingInvoiced:
		return "Awaiting payment"
	case registration.BillingBalancePaid:
		return "Paid in full"
	case registration.BillingReleased:
		return "Released"
	default:
		return status
	}
}

type groupReader interface {
	FindGroupByID(ctx context.Context, groupID string) (*registration.Group, error)
}

func staleConflict(ctx context.Context, repo groupReader, groupID string) httpx.APIError {
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
	return httpx.Conflict("stale_state", msg, nil)
}

func mapVersionErr(ctx context.Context, repo groupReader, groupID string, err error) error {
	if errors.Is(err, registration.ErrVersionConflict) {
		return staleConflict(ctx, repo, groupID)
	}
	return httpx.Internal(err.Error())
}
