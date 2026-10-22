package camperedit

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

func (s *Service) RemoveCamper(ctx context.Context, groupID, camperID, actor string, expectedVersion int) (RemoveCamperSummary, error) {
	g, err := s.reads.FindGroupByID(ctx, groupID)
	if err != nil {
		return RemoveCamperSummary{}, fmt.Errorf("operation: %w", err)
	}
	if g == nil {
		return RemoveCamperSummary{}, domain.ErrGroupNotFound
	}

	campers, err := s.reads.CampersForGroup(ctx, groupID)
	if err != nil {
		return RemoveCamperSummary{}, fmt.Errorf("operation: %w", err)
	}

	target, err := domain.FindCamper(campers, camperID)
	if err != nil {
		return RemoveCamperSummary{}, err
	}

	summary := RemoveCamperSummary{CamperName: target.FullName()}

	if err := g.CanRemoveCamper(*target, campers); err != nil {
		return RemoveCamperSummary{}, err
	}

	invoiceStale := false
	if g.HasOpenInvoice() {
		if err := s.invoices.VoidInvoiceIdempotent(ctx, *g.BalanceInvoiceRef); err != nil {
			return RemoveCamperSummary{}, apperr.Invalidf("invoice void failed; camper was not removed: %v", err)
		}
		summary.InvoiceVoided = true
		invoiceStale = true
	}
	after := make([]domain.Camper, 0, len(campers)-1)
	for _, c := range campers {
		if c.ID != camperID {
			after = append(after, c)
		}
	}
	tr, _ := g.RosterChanged(after, invoiceStale)

	// If removing this camper leaves no coach passengers, void any open (unpaid)
	// separate coach invoice so a departed passenger's fee can't still be paid.
	if g.HasUnpaidCoachInvoice() {
		remainingCoach := int64(0)
		for _, c := range campers {
			if c.ID == camperID {
				continue
			}
			if c.CoachEligible() {
				remainingCoach++
			}
		}
		if remainingCoach == 0 {
			if err := s.invoices.VoidInvoiceIdempotent(ctx, *g.CoachInvoiceRef); err != nil {
				log.Printf("billing: void coach invoice after camper remove %s: %v", groupID, err)
			}
		}
	}

	meta := domain.ActionMeta{
		Actor:           actor,
		Action:          "camper_removed",
		ExpectedVersion: expectedVersion,
	}
	if err := s.repo.DeleteCamperMeta(ctx, groupID, camperID, meta, tr); err != nil {
		return RemoveCamperSummary{}, billing.MapVersionErr(ctx, s.reads, groupID, err)
	}

	if g.PaymentStatus == domain.PaymentPaid {
		remaining, err := s.reads.CampersForGroup(ctx, groupID)
		if err != nil {
			log.Printf("billing: load campers after remove %s/%s: %v", groupID, camperID, err)
		} else {
			gAfter, err := s.reads.FindGroupByID(ctx, groupID)
			if err != nil || gAfter == nil {
				log.Printf("billing: reload group after remove %s: %v", groupID, err)
			} else {
				if err := s.sheets.RemoveByGroupID(ctx, groupID); err != nil {
					log.Printf("billing: remove group %s from sheet after camper remove: %v", groupID, err)
				}
				rows := billing.SheetRowsForGroup(gAfter, remaining)
				if err := s.sheets.AppendPaidAndRemovePending(ctx, groupID, rows); err != nil {
					log.Printf("billing: re-sync sheet for group %s after camper remove: %v", groupID, err)
				}
			}
		}
	}

	return summary, nil
}

func (s *Service) ConvertCamperToDayVisitor(
	ctx context.Context,
	groupID, camperID, actor string,
	expectedVersion int,
	req ConvertToDayVisitorRequest,
) (ConvertSummary, error) {
	g, err := s.reads.FindGroupByID(ctx, groupID)
	if err != nil {
		return ConvertSummary{}, fmt.Errorf("operation: %w", err)
	}
	if g == nil {
		return ConvertSummary{}, domain.ErrGroupNotFound
	}

	campers, err := s.reads.CampersForGroup(ctx, groupID)
	if err != nil {
		return ConvertSummary{}, fmt.Errorf("operation: %w", err)
	}

	target, err := domain.FindCamper(campers, camperID)
	if err != nil {
		return ConvertSummary{}, err
	}

	summary := ConvertSummary{CamperName: target.FullName()}

	if err := g.CanConvertToDayVisitor(*target); err != nil {
		return ConvertSummary{}, err
	}

	fields := map[string]string{}
	domain.ValidateDayPass("attendance", domain.Attendance{
		Type:          domain.AttendanceDayPass,
		Days:          req.Days,
		TshirtOption:  req.TshirtOption,
		ShirtSize:     req.ShirtSize,
		NeedsCatering: req.NeedsCatering,
	}, fields)
	if len(fields) > 0 {
		return ConvertSummary{}, apperr.Validation(fields)
	}

	// A camper added after the group's deposit checkout has their deposit
	// recorded as owed rather than paid, so there is nothing to credit back —
	// crediting it would hand the family money they never gave us. Clearing what
	// they owe (done in the same write) is the whole of the refund.
	depositCreditPence := 0
	if g.PaymentStatus == domain.PaymentPaid &&
		target.Age >= domain.MinDepositAge &&
		target.DepositOwedPence == 0 {
		depositCreditPence = s.cfg.DepositPricePence
	}
	summary.DepositCreditPence = depositCreditPence

	if depositCreditPence > 0 {
		customerID := ""
		if g.BillingCustomerRef != nil {
			customerID = *g.BillingCustomerRef
		}
		name := strings.TrimSpace(g.ContactFirstName + " " + g.ContactLastName)
		customerID, err = s.invoices.EnsureCustomer(ctx, customerID, g.ContactEmail, name, g.ID)
		if err != nil {
			return ConvertSummary{}, fmt.Errorf("operation: %w", err)
		}
		if customerID != "" && (g.BillingCustomerRef == nil || *g.BillingCustomerRef != customerID) {
			if err := s.repo.SetBillingCustomerRef(ctx, groupID, customerID); err != nil {
				return ConvertSummary{}, fmt.Errorf("operation: %w", err)
			}
		}
		if err := s.invoices.CreditCustomerBalance(
			ctx, customerID, int64(depositCreditPence), g.Currency,
			"Deposit credit (converted to day visitor)", "deposit-credit-"+camperID,
		); err != nil {
			return ConvertSummary{}, fmt.Errorf("operation: %w", err)
		}
	}

	invoiceStale := false
	if g.HasOpenInvoice() {
		if err := s.invoices.VoidInvoiceIdempotent(ctx, *g.BalanceInvoiceRef); err != nil {
			return ConvertSummary{}, apperr.Invalidf("invoice void failed; camper was not converted: %v", err)
		}
		summary.InvoiceVoided = true
		invoiceStale = true
	}
	after := make([]domain.Camper, 0, len(campers)-1)
	for _, c := range campers {
		if c.ID != camperID {
			after = append(after, c)
		}
	}
	tr, _ := g.RosterChanged(after, invoiceStale)

	var shirtSize *string
	switch req.TshirtOption {
	case domain.TshirtOptionNone:
		na := domain.ShirtSizeNotApplicable
		shirtSize = &na
	default:
		ss := strings.TrimSpace(req.ShirtSize)
		shirtSize = &ss
	}
	needsCatering := false
	if req.NeedsCatering != nil {
		needsCatering = *req.NeedsCatering
	}

	dp := domain.DayPassFields{
		Days:          req.Days,
		TshirtOption:  req.TshirtOption,
		ShirtSize:     shirtSize,
		NeedsCatering: needsCatering,
		Dietary:       target.DietaryRequirements,
	}

	meta := domain.ActionMeta{
		Actor:           actor,
		Action:          "camper_converted",
		ExpectedVersion: expectedVersion,
	}
	if err := s.repo.ConvertCamperToDayPassMeta(ctx, groupID, camperID, dp, depositCreditPence, meta, tr); err != nil {
		return ConvertSummary{}, billing.MapVersionErr(ctx, s.reads, groupID, err)
	}

	if g.PaymentStatus == domain.PaymentPaid {
		remaining, err := s.reads.CampersForGroup(ctx, groupID)
		if err != nil {
			log.Printf("billing: load campers after convert %s/%s: %v", groupID, camperID, err)
		} else {
			gAfter, err := s.reads.FindGroupByID(ctx, groupID)
			if err != nil || gAfter == nil {
				log.Printf("billing: reload group after convert %s: %v", groupID, err)
			} else {
				if err := s.sheets.RemoveByGroupID(ctx, groupID); err != nil {
					log.Printf("billing: remove group %s from sheet after camper convert: %v", groupID, err)
				}
				rows := billing.SheetRowsForGroup(gAfter, remaining)
				if err := s.sheets.AppendPaidAndRemovePending(ctx, groupID, rows); err != nil {
					log.Printf("billing: re-sync sheet for group %s after camper convert: %v", groupID, err)
				}
			}
		}
	}

	return summary, nil
}

func (s *Service) UpdateDayPassCamper(
	ctx context.Context,
	groupID, camperID, actor string,
	expectedVersion int,
	req UpdateDayPassCamperRequest,
) (UpdateDayPassSummary, error) {
	g, err := s.reads.FindGroupByID(ctx, groupID)
	if err != nil {
		return UpdateDayPassSummary{}, fmt.Errorf("operation: %w", err)
	}
	if g == nil {
		return UpdateDayPassSummary{}, domain.ErrGroupNotFound
	}

	campers, err := s.reads.CampersForGroup(ctx, groupID)
	if err != nil {
		return UpdateDayPassSummary{}, fmt.Errorf("operation: %w", err)
	}

	target, err := domain.FindCamper(campers, camperID)
	if err != nil {
		return UpdateDayPassSummary{}, err
	}

	summary := UpdateDayPassSummary{CamperName: target.FullName()}

	if err := target.CanEditDayPass(); err != nil {
		return UpdateDayPassSummary{}, err
	}

	fields := map[string]string{}
	domain.ValidateDayPass("attendance", domain.Attendance{
		Type:          domain.AttendanceDayPass,
		Days:          target.DayPassDays,
		TshirtOption:  req.TshirtOption,
		ShirtSize:     req.ShirtSize,
		NeedsCatering: req.NeedsCatering,
	}, fields)
	if len(fields) > 0 {
		return UpdateDayPassSummary{}, apperr.Validation(fields)
	}

	var shirtSize *string
	switch req.TshirtOption {
	case domain.TshirtOptionNone:
		na := domain.ShirtSizeNotApplicable
		shirtSize = &na
	default:
		ss := strings.TrimSpace(req.ShirtSize)
		shirtSize = &ss
	}
	needsCatering := false
	if req.NeedsCatering != nil {
		needsCatering = *req.NeedsCatering
	}

	var dietary *string
	if req.Dietary != nil {
		d := strings.TrimSpace(*req.Dietary)
		dietary = &d
	}

	meta := domain.ActionMeta{
		Actor:           actor,
		Action:          "camper_updated",
		ExpectedVersion: expectedVersion,
	}
	if err := s.repo.UpdateDayPassCamperMeta(
		ctx, groupID, camperID, req.TshirtOption, needsCatering, shirtSize, dietary, meta,
	); err != nil {
		return UpdateDayPassSummary{}, billing.MapVersionErr(ctx, s.reads, groupID, err)
	}

	if g.PaymentStatus == domain.PaymentPaid {
		remaining, err := s.reads.CampersForGroup(ctx, groupID)
		if err != nil {
			log.Printf("billing: load campers after day-pass edit %s/%s: %v", groupID, camperID, err)
		} else {
			gAfter, err := s.reads.FindGroupByID(ctx, groupID)
			if err != nil || gAfter == nil {
				log.Printf("billing: reload group after day-pass edit %s: %v", groupID, err)
			} else {
				if err := s.sheets.RemoveByGroupID(ctx, groupID); err != nil {
					log.Printf("billing: remove group %s from sheet after day-pass edit: %v", groupID, err)
				}
				rows := billing.SheetRowsForGroup(gAfter, remaining)
				if err := s.sheets.AppendPaidAndRemovePending(ctx, groupID, rows); err != nil {
					log.Printf("billing: re-sync sheet for group %s after day-pass edit: %v", groupID, err)
				}
			}
		}
	}

	return summary, nil
}

func (s *Service) UpdateCamperCoach(
	ctx context.Context,
	groupID, camperID, actor string,
	expectedVersion int,
	req UpdateCamperCoachRequest,
) (UpdateCamperCoachSummary, error) {
	g, err := s.reads.FindGroupByID(ctx, groupID)
	if err != nil {
		return UpdateCamperCoachSummary{}, fmt.Errorf("operation: %w", err)
	}
	if g == nil {
		return UpdateCamperCoachSummary{}, domain.ErrGroupNotFound
	}

	campers, err := s.reads.CampersForGroup(ctx, groupID)
	if err != nil {
		return UpdateCamperCoachSummary{}, fmt.Errorf("operation: %w", err)
	}

	target, err := domain.FindCamper(campers, camperID)
	if err != nil {
		return UpdateCamperCoachSummary{}, err
	}

	summary := UpdateCamperCoachSummary{
		CamperName: target.FullName(),
		NeedsCoach: req.NeedsCoach,
	}

	noOp, err := g.CoachChange(*target, req.NeedsCoach)
	if err != nil {
		return UpdateCamperCoachSummary{}, err
	}
	if noOp {
		summary.NoOp = true
		return summary, nil
	}

	invoiceVoided := false
	if g.HasUnpaidCoachInvoice() {
		if err := s.invoices.VoidInvoiceIdempotent(ctx, *g.CoachInvoiceRef); err != nil {
			return UpdateCamperCoachSummary{}, apperr.Invalidf("coach invoice void failed; coach seat was not updated: %v", err)
		}
		invoiceVoided = true
	}
	summary.InvoiceVoided = invoiceVoided

	meta := domain.ActionMeta{
		Actor:           actor,
		Action:          "camper_coach_updated",
		ExpectedVersion: expectedVersion,
	}
	if err := s.repo.UpdateCamperCoachMeta(ctx, groupID, camperID, req.NeedsCoach, invoiceVoided, meta); err != nil {
		return UpdateCamperCoachSummary{}, billing.MapVersionErr(ctx, s.reads, groupID, err)
	}

	if g.PaymentStatus == domain.PaymentPaid {
		remaining, err := s.reads.CampersForGroup(ctx, groupID)
		if err != nil {
			log.Printf("billing: load campers after coach edit %s/%s: %v", groupID, camperID, err)
		} else {
			gAfter, err := s.reads.FindGroupByID(ctx, groupID)
			if err != nil || gAfter == nil {
				log.Printf("billing: reload group after coach edit %s: %v", groupID, err)
			} else {
				if err := s.sheets.RemoveByGroupID(ctx, groupID); err != nil {
					log.Printf("billing: remove group %s from sheet after coach edit: %v", groupID, err)
				}
				rows := billing.SheetRowsForGroup(gAfter, remaining)
				if err := s.sheets.AppendPaidAndRemovePending(ctx, groupID, rows); err != nil {
					log.Printf("billing: re-sync sheet for group %s after coach edit: %v", groupID, err)
				}
			}
		}
	}

	return summary, nil
}

func (s *Service) EditCamper(
	ctx context.Context,
	groupID, camperID, actor string,
	expectedVersion int,
	req EditCamperRequest,
) (EditCamperSummary, error) {
	g, err := s.reads.FindGroupByID(ctx, groupID)
	if err != nil {
		return EditCamperSummary{}, fmt.Errorf("operation: %w", err)
	}
	if g == nil {
		return EditCamperSummary{}, domain.ErrGroupNotFound
	}
	campers, err := s.reads.CampersForGroup(ctx, groupID)
	if err != nil {
		return EditCamperSummary{}, fmt.Errorf("operation: %w", err)
	}
	target, err := domain.FindCamper(campers, camperID)
	if err != nil {
		return EditCamperSummary{}, err
	}

	fullWeek := target.AttendanceType == domain.AttendanceFullWeek
	if err := validateEdit(*target, req, fullWeek); err != nil {
		return EditCamperSummary{}, err
	}

	details := domain.CamperDetails{
		FirstName:      strings.TrimSpace(req.FirstName),
		LastName:       strings.TrimSpace(req.LastName),
		Gender:         req.Gender,
		Age:            req.Age,
		CellLeaderName: strings.TrimSpace(req.CellLeaderName),
		IsCellLeader:   req.IsCellLeader,
	}

	repriced := false
	if fullWeek {
		allocCode := strings.TrimSpace(req.AllocatedAccommodationCode)
		unitCode := strings.TrimSpace(req.AllocatedUnitCode)
		if allocCode != "" {
			if err := s.pricer.ValidateAllocatedUnit(ctx, allocCode, unitCode); err != nil {
				return EditCamperSummary{}, err
			}
		} else if err := domain.ValidateAllocationChoice(allocCode, unitCode); err != nil {
			return EditCamperSummary{}, err
		}

		priceBefore, err := s.pricer.PriceFor(ctx, billing.StrVal(target.AllocatedAccommodationCode), target.Age, g.IsFree)
		if err != nil {
			return EditCamperSummary{}, err
		}
		priceAfter, err := s.pricer.PriceFor(ctx, allocCode, req.Age, g.IsFree)
		if err != nil {
			return EditCamperSummary{}, err
		}
		repriced = priceBefore != priceAfter ||
			target.CoachEligible() != target.CoachEligibleAtAge(req.Age)

		details.ShirtSize = optStr(req.ShirtSize)
		details.DietaryRequirements = optStr(req.DietaryRequirements)
		details.AccommodationFirstChoice = optStr(req.AccommodationFirstChoice)
		details.AccommodationSecondChoice = optStr(req.AccommodationSecondChoice)
		details.AllocatedAccommodationCode = optStr(allocCode)
		details.AllocatedUnitCode = optStr(unitCode)
		details.BilledPriceRef = optStr(priceAfter)
	}

	if err := g.CheckReprice(repriced); err != nil {
		return EditCamperSummary{}, err
	}

	summary := EditCamperSummary{
		CamperName:   strings.TrimSpace(details.FirstName + " " + details.LastName),
		PreviousName: target.FullName(),
		Repriced:     repriced,
	}

	invoiceStale := false
	if repriced && g.HasOpenInvoice() {
		if err := s.invoices.VoidInvoiceIdempotent(ctx, *g.BalanceInvoiceRef); err != nil {
			return EditCamperSummary{}, apperr.Invalidf("invoice void failed; nothing was changed: %v", err)
		}
		summary.InvoiceVoided = true
		invoiceStale = true
	}
	after := campersAfterEdit(campers, camperID, details)
	tr, _ := g.RosterChanged(after, invoiceStale)

	meta := domain.ActionMeta{
		Actor:           actor,
		Action:          "camper_edited",
		ExpectedVersion: expectedVersion,
	}
	if err := s.repo.UpdateCamperDetailsMeta(ctx, groupID, camperID, details, meta, tr); err != nil {
		return EditCamperSummary{}, billing.MapVersionErr(ctx, s.reads, groupID, err)
	}

	s.resyncSheet(ctx, g, "camper edit")
	s.sendBookingUpdated(ctx, groupID, g, "", summary.InvoiceVoided)
	return summary, nil
}

func (s *Service) AddCamper(
	ctx context.Context,
	groupID, actor string,
	expectedVersion int,
	req AddCamperRequest,
) (AddCamperSummary, error) {
	g, err := s.reads.FindGroupByID(ctx, groupID)
	if err != nil {
		return AddCamperSummary{}, fmt.Errorf("operation: %w", err)
	}
	if g == nil {
		return AddCamperSummary{}, domain.ErrGroupNotFound
	}

	if err := g.CanAddCamper(); err != nil {
		return AddCamperSummary{}, err
	}

	c := req.Camper
	c.IsMainContact = false
	fields := map[string]string{}
	domain.ValidateCamper("camper", c, fields)
	if len(fields) > 0 {
		return AddCamperSummary{}, apperr.Validation(fields)
	}

	depositOwed := 0
	if c.Attendance.Type == domain.AttendanceFullWeek && c.Age >= domain.MinDepositAge {
		depositOwed = s.pricer.DepositAmountPence(ctx)
	}

	summary := AddCamperSummary{
		CamperName:       strings.TrimSpace(c.FirstName + " " + c.LastName),
		DepositOwedPence: depositOwed,
	}

	campers, err := s.reads.CampersForGroup(ctx, groupID)
	if err != nil {
		return AddCamperSummary{}, fmt.Errorf("operation: %w", err)
	}

	// A new full-week camper has no bed. Sending the group back to needing
	// accommodation is what puts them in front of the White Team again, rather
	// than leaving someone quietly unplaced on an allocated group.
	addedFullWeek := c.Attendance.Type == domain.AttendanceFullWeek
	invoiceStale := false
	if g.HasOpenInvoice() {
		if err := s.invoices.VoidInvoiceIdempotent(ctx, *g.BalanceInvoiceRef); err != nil {
			return AddCamperSummary{}, apperr.Invalidf("invoice void failed; nobody was added: %v", err)
		}
		summary.InvoiceVoided = true
		invoiceStale = true
	}
	after := append([]domain.Camper{}, campers...)
	after = append(after, domain.Camper{AttendanceType: c.Attendance.Type})
	tr, _ := g.RosterChanged(after, invoiceStale)
	summary.NeedsAllocation = addedFullWeek

	meta := domain.ActionMeta{
		Actor:           actor,
		Action:          "camper_added",
		ExpectedVersion: expectedVersion,
	}
	camperID, err := s.repo.AddCamperMeta(ctx, groupID, c, depositOwed, meta, tr)
	if err != nil {
		return AddCamperSummary{}, billing.MapVersionErr(ctx, s.reads, groupID, err)
	}
	summary.CamperID = camperID

	s.resyncSheet(ctx, g, "camper add")
	s.sendBookingUpdated(ctx, groupID, g, summary.CamperName, summary.InvoiceVoided)
	return summary, nil
}

func (s *Service) MakeMainContact(
	ctx context.Context,
	groupID, camperID, actor string,
	expectedVersion int,
) (string, error) {
	g, err := s.reads.FindGroupByID(ctx, groupID)
	if err != nil {
		return "", fmt.Errorf("operation: %w", err)
	}
	if g == nil {
		return "", domain.ErrGroupNotFound
	}
	campers, err := s.reads.CampersForGroup(ctx, groupID)
	if err != nil {
		return "", fmt.Errorf("operation: %w", err)
	}
	target, err := domain.FindCamper(campers, camperID)
	if err != nil {
		return "", err
	}
	name := target.FullName()
	if target.IsMainContact {
		return name, nil
	}

	meta := domain.ActionMeta{
		Actor:           actor,
		Action:          "main_contact_moved",
		ExpectedVersion: expectedVersion,
	}
	if err := s.repo.SetMainContactMeta(ctx, groupID, camperID, meta); err != nil {
		return "", billing.MapVersionErr(ctx, s.reads, groupID, err)
	}
	s.resyncSheet(ctx, g, "main contact change")
	return name, nil
}

func (s *Service) WaiveCamperDeposit(
	ctx context.Context,
	groupID, camperID, actor string,
	expectedVersion int,
) (string, error) {
	g, err := s.reads.FindGroupByID(ctx, groupID)
	if err != nil {
		return "", fmt.Errorf("operation: %w", err)
	}
	if g == nil {
		return "", domain.ErrGroupNotFound
	}
	if err := g.CanWaiveCamperDeposit(); err != nil {
		return "", err
	}
	campers, err := s.reads.CampersForGroup(ctx, groupID)
	if err != nil {
		return "", fmt.Errorf("operation: %w", err)
	}
	target, err := domain.FindCamper(campers, camperID)
	if err != nil {
		return "", err
	}
	name := target.FullName()
	if target.DepositOwedPence <= 0 {
		return name, nil
	}

	meta := domain.ActionMeta{
		Actor:           actor,
		Action:          "camper_deposit_waived",
		ExpectedVersion: expectedVersion,
	}
	if err := s.repo.WaiveCamperDepositMeta(ctx, groupID, camperID, meta); err != nil {
		return "", billing.MapVersionErr(ctx, s.reads, groupID, err)
	}
	return name, nil
}

func validateEdit(target domain.Camper, req EditCamperRequest, fullWeek bool) error {
	c := domain.NewCamper{
		FirstName:      req.FirstName,
		LastName:       req.LastName,
		Gender:         req.Gender,
		Age:            req.Age,
		CellLeaderName: req.CellLeaderName,
		IsCellLeader:   req.IsCellLeader,
	}
	if fullWeek {
		c.Attendance = domain.Attendance{
			Type:                      domain.AttendanceFullWeek,
			ShirtSize:                 req.ShirtSize,
			DietaryRequirements:       req.DietaryRequirements,
			AccommodationFirstChoice:  req.AccommodationFirstChoice,
			AccommodationSecondChoice: req.AccommodationSecondChoice,
			RoommateRequests:          billing.StrVal(target.RoommateRequests),
			NeedsCoach:                target.NeedsCoach,
		}
	} else {
		// The day-pass fields are not editable here, so they are carried over
		// from the stored row to be validated as a whole with the new age.
		c.Attendance = domain.Attendance{
			Type:                domain.AttendanceDayPass,
			Days:                target.DayPassDays,
			TshirtOption:        billing.StrVal(target.DayPassTshirtOption),
			ShirtSize:           billing.StrVal(target.ShirtSize),
			DietaryRequirements: billing.StrVal(target.DietaryRequirements),
			NeedsCatering:       target.DayPassNeedsCatering,
		}
	}

	fields := map[string]string{}
	domain.ValidateCamper("camper", c, fields)
	if fullWeek {
		domain.ValidateAllocatedAccommodation(
			"camper.allocated_accommodation_code",
			req.AllocatedAccommodationCode, req.Age, fields)
	}
	if len(fields) > 0 {
		return apperr.Validation(fields)
	}
	return nil
}

func campersAfterEdit(
	campers []domain.Camper,
	camperID string,
	details domain.CamperDetails,
) []domain.Camper {
	out := make([]domain.Camper, len(campers))
	copy(out, campers)
	for i := range out {
		if out[i].ID != camperID {
			continue
		}
		out[i].Age = details.Age
		if out[i].AttendanceType == domain.AttendanceFullWeek {
			out[i].AllocatedAccommodationCode = details.AllocatedAccommodationCode
			out[i].AllocatedUnitCode = details.AllocatedUnitCode
		}
	}
	return out
}

func optStr(s string) *string {
	t := strings.TrimSpace(s)
	if t == "" {
		return nil
	}
	return &t
}

func (s *Service) resyncSheet(ctx context.Context, g *domain.Group, what string) {
	if g.PaymentStatus != domain.PaymentPaid {
		return
	}
	campers, err := s.reads.CampersForGroup(ctx, g.ID)
	if err != nil {
		log.Printf("billing: load campers for sheet after %s (group %s): %v", what, g.ID, err)
		return
	}
	gAfter, err := s.reads.FindGroupByID(ctx, g.ID)
	if err != nil || gAfter == nil {
		log.Printf("billing: reload group for sheet after %s (group %s): %v", what, g.ID, err)
		return
	}
	if err := s.sheets.RemoveByGroupID(ctx, g.ID); err != nil {
		log.Printf("billing: clear sheet rows after %s (group %s): %v", what, g.ID, err)
	}
	if err := s.sheets.AppendPaidAndRemovePending(ctx, g.ID, billing.SheetRowsForGroup(gAfter, campers)); err != nil {
		log.Printf("billing: re-sync sheet after %s (group %s): %v", what, g.ID, err)
	}
}

func (s *Service) sendBookingUpdated(
	ctx context.Context,
	groupID string,
	g *domain.Group,
	addedName string,
	invoiceVoided bool,
) {
	if s.mailer == nil {
		return
	}
	campers, err := s.reads.CampersForGroup(ctx, groupID)
	if err != nil {
		log.Printf("billing: load campers for booking-updated email (group %s): %v", groupID, err)
		return
	}
	if err := s.mailer.SendBookingUpdated(ctx, email.BookingUpdated{
		ToEmail:          g.ContactEmail,
		ToName:           g.ContactFirstName,
		Campers:          billing.RosterItems(campers, s.pricer.AccommodationNames(ctx), s.pricer.UnitNames(ctx)),
		AddedName:        addedName,
		InvoiceCancelled: invoiceVoided,
	}); err != nil {
		log.Printf("billing: booking-updated email to %s: %v", g.ContactEmail, err)
	}
}
