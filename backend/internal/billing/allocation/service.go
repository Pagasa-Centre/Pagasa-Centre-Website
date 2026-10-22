package allocation

import (
	"context"
	"fmt"
	"log"
	"strings"

	"pagasacentre/backend/internal/billing"
	"pagasacentre/backend/internal/email"
	"pagasacentre/backend/internal/registration/domain"
	"pagasacentre/backend/pkg/commonlibrary/apperr"
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
	byID := map[string]domain.Camper{}
	for _, c := range campers {
		byID[c.ID] = c
	}

	var allocs []domain.CamperAllocation
	for _, a := range req.Campers {
		c, ok := byID[a.CamperID]
		if !ok {
			return apperr.Invalid("unknown camper_id " + a.CamperID)
		}
		if c.AttendanceType != domain.AttendanceFullWeek {
			continue
		}
		code := strings.TrimSpace(a.AllocatedAccommodationCode)
		if code == "" {
			return apperr.Validation(map[string]string{
				a.CamperID: "allocated_accommodation_code is required for full-week campers",
			})
		}
		priceID := strings.TrimSpace(a.BilledStripePriceID)
		if g.IsFree {
			priceID = ""
		} else if priceID == "" {
			priceID, err = s.pricer.ResolvePriceID(ctx, code, c.Age)
			if err != nil {
				return err
			}
		}
		unitCode := strings.TrimSpace(a.AllocatedUnitCode)
		if err := s.pricer.ValidateAllocatedUnit(ctx, code, unitCode); err != nil {
			return err
		}
		allocs = append(allocs, domain.CamperAllocation{
			CamperID:                   a.CamperID,
			AllocatedAccommodationCode: code,
			AllocatedUnitCode:          unitCode,
			BilledStripePriceID:        priceID,
		})
	}

	// Every full-week camper must be allocated.
	for _, c := range campers {
		if c.AttendanceType != domain.AttendanceFullWeek {
			continue
		}
		found := false
		for _, a := range allocs {
			if a.CamperID == c.ID {
				found = true
				break
			}
		}
		if !found {
			return apperr.Validation(map[string]string{
				"campers": fmt.Sprintf("full-week camper %s %s is not allocated", c.FirstName, c.LastName),
			})
		}
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
	if err := s.mailer.SendSponsorshipConfirmed(ctx, email.SponsorshipConfirmed{
		ToEmail: g.ContactEmail,
		ToName:  g.ContactFirstName,
		Items:   billing.BalanceInvoiceItems(campers, accNames, s.pricer.UnitNames(ctx), 0),
		Changes: billing.OffPreferenceChanges(campers, accNames),
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
	if g.StripeInvoiceID != nil && *g.StripeInvoiceID != "" &&
		g.BillingStatus == domain.BillingInvoiced {
		if err := s.stripe.VoidInvoice(ctx, *g.StripeInvoiceID); err != nil {
			log.Printf("billing: void invoice %s for group %s: %v", *g.StripeInvoiceID, groupID, err)
			// Continue — DB release still needed if Stripe already void/paid.
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
	names := camperNames(campers)
	if err := s.mailer.SendAllocationReleased(ctx, email.AllocationReleased{
		ToEmail:     g.ContactEmail,
		ToName:      g.ContactFirstName,
		CamperNames: names,
		Reason:      reason,
		Cancelled:   cancelled,
	}); err != nil {
		log.Printf("billing: allocation released email to %s: %v", g.ContactEmail, err)
	}
	if s.cfg.WhiteTeamEmail != "" {
		_ = s.mailer.SendWhiteTeamNotification(ctx, email.WhiteTeamNotification{
			ToEmail: s.cfg.WhiteTeamEmail,
			Subject: "Camp allocation released (unpaid)",
			Body: fmt.Sprintf(
				"Group %s (%s) — allocation released.\nReason: %s\nCampers: %s",
				groupID, g.ContactEmail, reason, strings.Join(names, ", ")),
		})
	}
	return nil
}

func camperNames(campers []domain.Camper) []string {
	var names []string
	for _, c := range campers {
		names = append(names, c.FirstName+" "+c.LastName)
	}
	return names
}
