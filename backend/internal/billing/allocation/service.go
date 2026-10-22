package allocation

import (
	"context"
	"fmt"
	"log"
	"strings"

	"pagasacentre/backend/internal/billing"
	"pagasacentre/backend/internal/notify"
	"pagasacentre/backend/internal/registration/domain"
)

func (s *Service) Allocate(ctx context.Context, groupID, actor string, expectedVersion int, req AllocateRequest) error {
	g, err := s.reads.FindGroupByID(ctx, groupID)
	if err != nil {
		return fmt.Errorf("operation: %w", err)
	}
	if g == nil {
		return domain.ErrGroupNotFound
	}
	tr, err := g.Allocate()
	if err != nil {
		return err
	}

	campers, err := s.reads.CampersForGroup(ctx, groupID)
	if err != nil {
		return fmt.Errorf("operation: %w", err)
	}

	var allocs []domain.CamperAllocation
	for _, a := range req.Campers {
		c, skip, err := domain.AllocationTarget(campers, a.CamperID, a.AllocatedAccommodationCode)
		if err != nil {
			return err
		}
		if skip {
			continue
		}
		code := strings.TrimSpace(a.AllocatedAccommodationCode)
		priceID := strings.TrimSpace(a.BilledPriceRef)
		ref, needsLookup := g.AllocationPriceRef(priceID)
		if needsLookup {
			priceID, err = s.pricer.ResolvePriceID(ctx, code, c.Age)
			if err != nil {
				return err
			}
		} else {
			priceID = ref
		}
		unitCode := strings.TrimSpace(a.AllocatedUnitCode)
		if err := s.pricer.ValidateAllocatedUnit(ctx, code, unitCode); err != nil {
			return err
		}
		allocs = append(allocs, domain.CamperAllocation{
			CamperID:                   a.CamperID,
			AllocatedAccommodationCode: code,
			AllocatedUnitCode:          unitCode,
			BilledPriceRef:             priceID,
		})
	}

	if err := domain.CheckAllCampersAllocated(campers, allocs); err != nil {
		return err
	}

	meta := domain.ActionMeta{
		Actor:           actor,
		Action:          "allocated",
		ExpectedVersion: expectedVersion,
	}
	if err := s.repo.AllocateGroup(ctx, groupID, allocs, meta, tr); err != nil {
		return billing.MapVersionErr(ctx, s.reads, groupID, err)
	}
	return nil
}

func (s *Service) Unallocate(ctx context.Context, groupID, actor string, expectedVersion int) error {
	g, err := s.reads.FindGroupByID(ctx, groupID)
	if err != nil {
		return fmt.Errorf("operation: %w", err)
	}
	if g == nil {
		return domain.ErrGroupNotFound
	}
	tr, err := g.Unallocate()
	if err != nil {
		return err
	}
	meta := domain.ActionMeta{
		Actor:           actor,
		Action:          "unallocated",
		ExpectedVersion: expectedVersion,
	}
	if err := s.repo.UnallocateGroup(ctx, groupID, meta, tr); err != nil {
		return billing.MapVersionErr(ctx, s.reads, groupID, err)
	}
	return nil
}

func (s *Service) ConfirmFree(ctx context.Context, groupID, actor string, expectedVersion int) error {
	g, err := s.reads.FindGroupByID(ctx, groupID)
	if err != nil {
		return fmt.Errorf("operation: %w", err)
	}
	if g == nil {
		return domain.ErrGroupNotFound
	}
	tr, err := g.ConfirmSponsorship()
	if err != nil {
		return err
	}
	meta := domain.ActionMeta{
		Actor:           actor,
		Action:          "free_confirmed",
		ExpectedVersion: expectedVersion,
	}
	if err := s.repo.ConfirmFreeMeta(ctx, groupID, meta, tr); err != nil {
		return billing.MapVersionErr(ctx, s.reads, groupID, err)
	}
	s.sendSponsorshipConfirmedEmail(ctx, g)
	return nil
}

func (s *Service) sendSponsorshipConfirmedEmail(ctx context.Context, g *domain.Group) {
	if s.mailer == nil {
		return
	}
	campers, err := s.reads.CampersForGroup(ctx, g.ID)
	if err != nil {
		log.Printf("billing: sponsorship email load campers (group %s): %v", g.ID, err)
		return
	}
	accNames := s.pricer.AccommodationNames(ctx)
	if err := s.mailer.SendSponsorshipConfirmed(ctx, notify.SponsorshipConfirmed{
		ToEmail: g.ContactEmail,
		ToName:  g.ContactFirstName,
		Items:   notify.BalanceItems(campers, accNames, s.pricer.UnitNames(ctx), 0),
		Changes: notify.OffPreferenceChanges(campers, accNames),
	}); err != nil {
		log.Printf("billing: sponsorship confirmed email to %s: %v", g.ContactEmail, err)
	}
}

func (s *Service) VoidAndRelease(ctx context.Context, groupID, reason, actor string, expectedVersion int, cancelled bool) error {
	g, err := s.reads.FindGroupByID(ctx, groupID)
	if err != nil {
		return fmt.Errorf("operation: %w", err)
	}
	if g == nil {
		return domain.ErrGroupNotFound
	}
	if g.HasOpenInvoice() {
		if g.BalanceInvoiceRef != nil && *g.BalanceInvoiceRef != "" {
			if err := s.invoices.VoidInvoice(ctx, *g.BalanceInvoiceRef); err != nil {
				log.Printf("billing: void invoice %s for group %s: %v", *g.BalanceInvoiceRef, groupID, err)
			}
		}
	}
	action := "released"
	if cancelled {
		action = "cancel"
	}
	tr := g.Release()
	meta := domain.ActionMeta{
		Actor:           actor,
		Action:          action,
		ExpectedVersion: expectedVersion,
	}
	if err := s.repo.ClearInvoiceAndReleaseMeta(ctx, groupID, meta, tr); err != nil {
		return billing.MapVersionErr(ctx, s.reads, groupID, err)
	}

	campers, _ := s.reads.CampersForGroup(ctx, groupID)
	names := notify.CamperNames(campers)
	if err := s.mailer.SendAllocationReleased(ctx, notify.AllocationReleased{
		ToEmail:     g.ContactEmail,
		ToName:      g.ContactFirstName,
		CamperNames: names,
		Reason:      reason,
		Cancelled:   cancelled,
	}); err != nil {
		log.Printf("billing: allocation released email to %s: %v", g.ContactEmail, err)
	}
	if s.cfg.WhiteTeamEmail != "" {
		_ = s.mailer.SendStaffAllocationReleased(ctx, notify.StaffAllocationReleased{
			ToEmail:      s.cfg.WhiteTeamEmail,
			GroupID:      groupID,
			ContactEmail: g.ContactEmail,
			Reason:       reason,
			CamperNames:  names,
		})
	}
	return nil
}
