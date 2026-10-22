package groupsql

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	"pagasacentre/backend/internal/registration/domain"
)

func SetCamperAllocationsTx(ctx context.Context, tx pgx.Tx, groupID string, allocs []domain.CamperAllocation) error {
	for _, a := range allocs {
		var unitCode any
		if strings.TrimSpace(a.AllocatedUnitCode) != "" {
			unitCode = strings.TrimSpace(a.AllocatedUnitCode)
		}
		tag, err := tx.Exec(ctx,
			`UPDATE registrations
			    SET allocated_accommodation_code = $1,
			        allocated_unit_code = $2,
			        billed_stripe_price_id = $3
			  WHERE id = $4 AND group_id = $5`,
			a.AllocatedAccommodationCode, unitCode, a.BilledPriceRef, a.CamperID, groupID)
		if err != nil {
			return fmt.Errorf("set allocation for camper %s: %w", a.CamperID, err)
		}
		if tag.RowsAffected() == 0 {
			return fmt.Errorf("camper %s not in group %s", a.CamperID, groupID)
		}
	}
	return nil
}
