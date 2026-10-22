package campadmin

import (
	"context"
	"strconv"
	"strings"
	"time"

	"pagasacentre/backend/internal/adminlog"
	"pagasacentre/backend/internal/billing/allocation"
	"pagasacentre/backend/internal/billing/camperedit"
	"pagasacentre/backend/internal/billing/invoicing"
)

func (s *Service) Allocate(ctx context.Context, groupID, actor string, expectedVersion int, req allocation.AllocateRequest) error {
	prior, _ := s.reg.FindGroup(ctx, groupID)
	campers, _ := s.reg.CampersForGroup(ctx, groupID)
	isEdit := prior != nil && prior.AllocationIsEdit(campers)

	if err := s.bill.Allocation.Allocate(ctx, groupID, actor, expectedVersion, req); err != nil {
		return err
	}
	g, _ := s.reg.FindGroup(ctx, groupID)
	gid := groupID
	if isEdit {
		s.record(ctx, actor, adminlog.ActionAllocationEdited, &gid,
			"Edited accommodation allocation for "+groupSummary(g), nil)
	} else {
		s.record(ctx, actor, adminlog.ActionAllocate, &gid,
			"Allocated accommodation for "+groupSummary(g), nil)
	}
	return nil
}

func (s *Service) SendInvoice(ctx context.Context, groupID, actor string, expectedVersion int) error {
	if err := s.bill.Invoicing.SendInvoice(ctx, groupID, actor, expectedVersion); err != nil {
		return err
	}
	g, _ := s.reg.FindGroup(ctx, groupID)
	gid := groupID
	s.record(ctx, actor, adminlog.ActionInvoiceSent, &gid,
		"Sent balance invoice to "+groupSummary(g), nil)
	return nil
}

func (s *Service) SendInvoicesBulk(ctx context.Context, actor string, groupIDs []string) map[string]error {
	errs := s.bill.Invoicing.SendInvoicesBulk(ctx, actor, groupIDs)
	for _, id := range groupIDs {
		if _, failed := errs[id]; failed {
			continue
		}
		g, _ := s.reg.FindGroup(ctx, id)
		gid := id
		s.record(ctx, actor, adminlog.ActionInvoiceSent, &gid,
			"Sent balance invoice to "+groupSummary(g), nil)
	}
	return errs
}

func (s *Service) SendCoachInvoice(ctx context.Context, groupID, actor string, expectedVersion int) error {
	if err := s.bill.Invoicing.SendCoachInvoice(ctx, groupID, actor, expectedVersion); err != nil {
		return err
	}
	g, _ := s.reg.FindGroup(ctx, groupID)
	gid := groupID
	s.record(ctx, actor, adminlog.ActionCoachInvoiceSent, &gid,
		"Sent coach invoice to "+groupSummary(g), nil)
	return nil
}

func (s *Service) SendCoachInvoicesBulk(ctx context.Context, actor string, groupIDs []string) map[string]error {
	errs := s.bill.Invoicing.SendCoachInvoicesBulk(ctx, actor, groupIDs)
	for _, id := range groupIDs {
		if _, failed := errs[id]; failed {
			continue
		}
		g, _ := s.reg.FindGroup(ctx, id)
		gid := id
		s.record(ctx, actor, adminlog.ActionCoachInvoiceSent, &gid,
			"Sent coach invoice to "+groupSummary(g), nil)
	}
	return errs
}

func (s *Service) WaiveCoachFee(ctx context.Context, groupID, actor string, expectedVersion int) error {
	if err := s.bill.Invoicing.WaiveCoachFee(ctx, groupID, actor, expectedVersion); err != nil {
		return err
	}
	g, _ := s.reg.FindGroup(ctx, groupID)
	gid := groupID
	s.record(ctx, actor, adminlog.ActionCoachFeeWaived, &gid,
		"Waived coach fee for "+groupSummary(g), nil)
	return nil
}

func (s *Service) UnwaiveCoachFee(ctx context.Context, groupID, actor string, expectedVersion int) error {
	if err := s.bill.Invoicing.UnwaiveCoachFee(ctx, groupID, actor, expectedVersion); err != nil {
		return err
	}
	g, _ := s.reg.FindGroup(ctx, groupID)
	gid := groupID
	s.record(ctx, actor, adminlog.ActionCoachFeeUnwaived, &gid,
		"Restored coach fee for "+groupSummary(g), nil)
	return nil
}

func (s *Service) Unallocate(ctx context.Context, groupID, actor string, expectedVersion int) error {
	if err := s.bill.Allocation.Unallocate(ctx, groupID, actor, expectedVersion); err != nil {
		return err
	}
	g, _ := s.reg.FindGroup(ctx, groupID)
	gid := groupID
	s.record(ctx, actor, adminlog.ActionUnallocate, &gid,
		"Reset allocation for "+groupSummary(g), nil)
	return nil
}

func (s *Service) Release(ctx context.Context, groupID, actor string, expectedVersion int, cancelled bool) error {
	reason, verb, action := "released by White Team", "Released", adminlog.ActionRelease
	if cancelled {
		reason, verb, action = "cancelled by White Team", "Cancelled", adminlog.ActionCancel
	}
	if err := s.bill.Allocation.VoidAndRelease(ctx, groupID, reason, actor, expectedVersion, cancelled); err != nil {
		return err
	}
	g, _ := s.reg.FindGroup(ctx, groupID)
	gid := groupID
	s.record(ctx, actor, action, &gid, verb+" allocation for "+groupSummary(g), nil)
	return nil
}

func (s *Service) MarkBalancePaidManually(ctx context.Context, groupID, actor string, expectedVersion int) error {
	if err := s.bill.Invoicing.MarkBalancePaidManually(ctx, groupID, actor, expectedVersion); err != nil {
		return err
	}
	g, _ := s.reg.FindGroup(ctx, groupID)
	gid := groupID
	s.record(ctx, actor, adminlog.ActionBalancePaid, &gid,
		"Marked balance paid outside Stripe for "+groupSummary(g), nil)
	return nil
}

func (s *Service) RemoveCamper(ctx context.Context, groupID, camperID, actor string, expectedVersion int) (camperedit.RemoveCamperSummary, error) {
	sum, err := s.bill.Campers.RemoveCamper(ctx, groupID, camperID, actor, expectedVersion)
	if err != nil {
		return sum, err
	}
	g, _ := s.reg.FindGroup(ctx, groupID)
	gid := groupID
	summary := "Removed " + sum.CamperName + " from " + groupSummary(g)
	if sum.InvoiceVoided {
		summary += "; voided open invoice"
	}
	s.record(ctx, actor, adminlog.ActionCamperRemoved, &gid, summary, sum)
	return sum, nil
}

func (s *Service) ConvertCamperToDayVisitor(ctx context.Context, groupID, camperID, actor string, expectedVersion int, req camperedit.ConvertToDayVisitorRequest) (camperedit.ConvertSummary, error) {
	sum, err := s.bill.Campers.ConvertCamperToDayVisitor(ctx, groupID, camperID, actor, expectedVersion, req)
	if err != nil {
		return sum, err
	}
	gid := groupID
	summary := "Converted " + sum.CamperName + " to day visitor"
	if sum.InvoiceVoided {
		summary += "; voided open invoice"
	}
	s.record(ctx, actor, adminlog.ActionCamperConverted, &gid, summary, sum)
	return sum, nil
}

func (s *Service) UpdateDayPassCamper(ctx context.Context, groupID, camperID, actor string, expectedVersion int, req camperedit.UpdateDayPassCamperRequest) (camperedit.UpdateDayPassSummary, error) {
	sum, err := s.bill.Campers.UpdateDayPassCamper(ctx, groupID, camperID, actor, expectedVersion, req)
	if err != nil {
		return sum, err
	}
	gid := groupID
	summary := "Updated day-pass details for " + sum.CamperName
	s.record(ctx, actor, adminlog.ActionCamperUpdated, &gid, summary, sum)
	return sum, nil
}

func (s *Service) UpdateCamperCoach(ctx context.Context, groupID, camperID, actor string, expectedVersion int, req camperedit.UpdateCamperCoachRequest) (camperedit.UpdateCamperCoachSummary, error) {
	sum, err := s.bill.Campers.UpdateCamperCoach(ctx, groupID, camperID, actor, expectedVersion, req)
	if err != nil {
		return sum, err
	}
	if sum.NoOp {
		return sum, nil
	}
	gid := groupID
	verb := "off"
	if sum.NeedsCoach {
		verb = "on"
	}
	summary := sum.CamperName + " coach seat turned " + verb
	if sum.InvoiceVoided {
		summary += "; voided open coach invoice"
	}
	s.record(ctx, actor, adminlog.ActionCamperCoachUpdated, &gid, summary, sum)
	return sum, nil
}

func (s *Service) EditCamper(ctx context.Context, groupID, camperID, actor string, expectedVersion int, req camperedit.EditCamperRequest) (camperedit.EditCamperSummary, error) {
	sum, err := s.bill.Campers.EditCamper(ctx, groupID, camperID, actor, expectedVersion, req)
	if err != nil {
		return sum, err
	}
	gid := groupID
	summary := "Edited " + sum.PreviousName
	if sum.CamperName != sum.PreviousName {
		summary = "Replaced " + sum.PreviousName + " with " + sum.CamperName
	}
	if sum.InvoiceVoided {
		summary += "; voided open invoice"
	}
	s.record(ctx, actor, adminlog.ActionCamperEdited, &gid, summary, sum)
	return sum, nil
}

func (s *Service) AddCamper(ctx context.Context, groupID, actor string, expectedVersion int, req camperedit.AddCamperRequest) (camperedit.AddCamperSummary, error) {
	sum, err := s.bill.Campers.AddCamper(ctx, groupID, actor, expectedVersion, req)
	if err != nil {
		return sum, err
	}
	gid := groupID
	summary := "Added " + sum.CamperName + " to the booking"
	if sum.DepositOwedPence > 0 {
		summary += "; owes a " + formatPence(sum.DepositOwedPence) + " deposit"
	}
	if sum.InvoiceVoided {
		summary += "; voided open invoice"
	}
	s.record(ctx, actor, adminlog.ActionCamperAdded, &gid, summary, sum)
	return sum, nil
}

func (s *Service) MakeMainContact(ctx context.Context, groupID, camperID, actor string, expectedVersion int) error {
	name, err := s.bill.Campers.MakeMainContact(ctx, groupID, camperID, actor, expectedVersion)
	if err != nil {
		return err
	}
	gid := groupID
	s.record(ctx, actor, adminlog.ActionMainContactMoved, &gid,
		"Made "+name+" the main contact", nil)
	return nil
}

func (s *Service) WaiveCamperDeposit(ctx context.Context, groupID, camperID, actor string, expectedVersion int) error {
	name, err := s.bill.Campers.WaiveCamperDeposit(ctx, groupID, camperID, actor, expectedVersion)
	if err != nil {
		return err
	}
	gid := groupID
	s.record(ctx, actor, adminlog.ActionCamperDepositWaived, &gid,
		"Waived the outstanding deposit for "+name, nil)
	return nil
}

func (s *Service) DeleteRegistration(ctx context.Context, groupID, actor string, expectedVersion int) (invoicing.DeleteSummary, error) {
	sum, err := s.bill.Invoicing.DeleteRegistration(ctx, groupID, actor, expectedVersion)
	if err != nil {
		return sum, err
	}
	summary := formatDeleteAuditSummary(sum)
	s.record(ctx, actor, adminlog.ActionRegistrationDeleted, nil, summary, sum)
	return sum, nil
}

func (s *Service) ResendInvoice(ctx context.Context, groupID, actor string, expectedVersion int) error {
	if err := s.bill.Invoicing.ResendInvoice(ctx, groupID, actor, expectedVersion); err != nil {
		return err
	}
	g, _ := s.reg.FindGroup(ctx, groupID)
	gid := groupID
	s.record(ctx, actor, adminlog.ActionInvoiceResent, &gid,
		"Re-sent invoice reminder to "+groupSummary(g), nil)
	return nil
}

func (s *Service) ResyncAllSheets(ctx context.Context, actor string) (int, map[string]error) {
	synced, errs := s.bill.Sheets.ResyncAllSheets(ctx, actor)
	summary := strings.TrimSpace(actor) + " re-synced " + strconv.Itoa(synced) + " group(s) to Google Sheet"
	s.record(ctx, actor, adminlog.ActionSheetResynced, nil, summary, map[string]any{
		"synced": synced,
		"errors": len(errs),
	})
	return synced, errs
}

func (s *Service) ExtendDueAt(ctx context.Context, groupID, actor string, expectedVersion int, dueAt time.Time) error {
	if err := s.bill.Invoicing.ExtendDueAt(ctx, groupID, actor, expectedVersion, dueAt); err != nil {
		return err
	}
	g, _ := s.reg.FindGroup(ctx, groupID)
	gid := groupID
	s.record(ctx, actor, adminlog.ActionExtendDue, &gid,
		"Extended invoice due date for "+groupSummary(g), nil)
	return nil
}

func (s *Service) SweepOverdue(ctx context.Context, actor string) (int, error) {
	n, err := s.bill.Sweep.SweepOverdue(ctx)
	if err != nil {
		return n, err
	}
	s.record(ctx, actor, adminlog.ActionSweep, nil,
		strings.TrimSpace(actor)+" released "+strconv.Itoa(n)+" overdue group(s)", map[string]int{"released": n})
	return n, nil
}

func (s *Service) ConfirmFree(ctx context.Context, groupID, actor string, expectedVersion int) error {
	if err := s.bill.Allocation.ConfirmFree(ctx, groupID, actor, expectedVersion); err != nil {
		return err
	}
	g, _ := s.reg.FindGroup(ctx, groupID)
	gid := groupID
	s.record(ctx, actor, adminlog.ActionFreeConfirmed, &gid,
		"Confirmed sponsorship for "+groupSummary(g), nil)
	return nil
}
