package billing

import (
	"context"
	"errors"
	"fmt"

	"pagasacentre/backend/internal/registration/domain"
	"pagasacentre/backend/pkg/commonlibrary/apperr"
)

func ExpectedVersion(v *int) int {
	if v == nil {
		return domain.SkipVersionCheck
	}
	return *v
}

func expectedVersion(v *int) int { return ExpectedVersion(v) }

// VersionReader loads a group for stale-state conflict messages.
type VersionReader interface {
	FindGroupByID(ctx context.Context, groupID string) (*domain.Group, error)
}

func StaleConflict(ctx context.Context, repo VersionReader, groupID string) *apperr.Error {
	who := "someone else"
	state := "updated"
	if g, err := repo.FindGroupByID(ctx, groupID); err == nil && g != nil {
		if g.LastActionBy != nil && *g.LastActionBy != "" {
			who = *g.LastActionBy
		}
		state = g.BillingStatus.Label()
	}
	msg := fmt.Sprintf(
		"This group was just updated by %s (%s). Your view was refreshed — please check and try again.",
		who, state,
	)
	return apperr.Conflict("stale_state", msg)
}

func MapVersionErr(ctx context.Context, repo VersionReader, groupID string, err error) error {
	if errors.Is(err, domain.ErrVersionConflict) {
		return StaleConflict(ctx, repo, groupID)
	}
	return fmt.Errorf("version check: %w", err)
}
