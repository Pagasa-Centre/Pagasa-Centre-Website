package invoicing

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"pagasacentre/backend/internal/billing"
	"pagasacentre/backend/internal/notify"
	"pagasacentre/backend/internal/registration/domain"
	"pagasacentre/backend/pkg/commonlibrary/apperr"
)

func (s *Service) SendInvoice(ctx context.Context, groupID, actor string, expectedVersion int) error {
	g, err := s.reads.FindGroupByID(ctx, groupID)
	if err != nil {
		return fmt.Errorf("operation: %w", err)
	}
	if g == nil {
		return domain.ErrGroupNotFound
	}
	campers, err := s.reads.CampersForGroup(ctx, groupID)
	if err != nil {
		return fmt.Errorf("operation: %w", err)
	}
	hasFullWeek := false
	for _, c := range campers {
		if c.AttendanceType == domain.AttendanceFullWeek {
			hasFullWeek = true
			break
		}
	}
	tr, err := g.Invoice(hasFullWeek)
	if err != nil {
		return err
	}

	var lines []billing.InvoiceLine
	for _, c := range campers {
		switch c.AttendanceType {
		case domain.AttendanceFullWeek:
			code, err := c.AllocatedForInvoice()
			if err != nil {
				return err
			}
			// Re-resolve the Stripe Price from the CURRENT accommodation config at
			// send time rather than replaying the value stored when the allocation
			// was first saved. This self-heals when Price IDs are corrected after
			// allocation (e.g. env vars set later), instead of erroring on a stale
			// "No such price".
			priceID, err := s.pricer.ResolvePriceID(ctx, code, c.Age)
			if err != nil {
				return err
			}
			lines = append(lines, billing.InvoiceLine{PriceID: priceID, Quantity: 1})
		case domain.AttendanceDayPass:
			line, ok := dayPassLine(c, s.cfg.DayPassPriceRef)
			if !ok {
				continue
			}
			if line.PriceID == "" {
				return apperr.Invalid("STRIPE_PRICE_DAY_PASS is not configured")
			}
			lines = append(lines, line)
		}
	}

	// Bill any deposit still owed by someone added after the group's deposit
	// checkout was settled. Charging it here, rather than chasing a second
	// payment, means a late arrival pays the same total as everyone who
	// registered on time, in one go.
	for _, c := range campers {
		if c.DepositOwedPence <= 0 {
			continue
		}
		lines = append(lines, billing.InvoiceLine{
			AmountPence: int64(c.DepositOwedPence),
			Currency:    g.Currency,
			Description: fmt.Sprintf("Camp deposit — %s %s", c.FirstName, c.LastName),
			Quantity:    1,
		})
	}

	// Fold the coach fee into this balance invoice for coach passengers. Only
	// groups that have not yet been invoiced reach here, so a coach fee that is
	// folded is always fresh (never double-charged). Skip when admin has waived.
	coachCount := domain.CoachEligibleCount(campers)
	coachIncluded := false
	if g.CoachFeeWaivedAt == nil {
		if line, ok := coachLine(coachCount, s.cfg.CoachPriceRef); ok {
			if line.PriceID == "" {
				return billing.ErrCoachPriceNotConfigured
			}
			lines = append(lines, line)
			coachIncluded = coachCount > 0
		}
	}

	if len(lines) == 0 {
		return apperr.Invalid("nothing to invoice")
	}

	customerID := ""
	if g.BillingCustomerRef != nil {
		customerID = *g.BillingCustomerRef
	}
	name := g.ContactName()
	customerID, err = s.invoices.EnsureCustomer(ctx, customerID, g.ContactEmail, name, g.ID)
	if err != nil {
		return fmt.Errorf("operation: %w", err)
	}
	if customerID != "" && (g.BillingCustomerRef == nil || *g.BillingCustomerRef != customerID) {
		if err := s.repo.SetBillingCustomerRef(ctx, groupID, customerID); err != nil {
			return fmt.Errorf("operation: %w", err)
		}
	}

	res, err := s.invoices.CreateInvoice(
		ctx, customerID, g.ID, lines, int64(s.cfg.InvoiceDueDays), "balance")
	if err != nil {
		return fmt.Errorf("operation: %w", err)
	}
	meta := domain.ActionMeta{
		Actor:           actor,
		Action:          "invoice_sent",
		ExpectedVersion: expectedVersion,
	}
	if err := s.repo.SetInvoiceDetailsMeta(ctx, groupID, res.ID, res.DueAt, coachIncluded, meta, tr); err != nil {
		return billing.MapVersionErr(ctx, s.reads, groupID, err)
	}
	accNames := s.pricer.AccommodationNames(ctx)
	changes := notify.OffPreferenceChanges(campers, accNames)
	s.sendAccommodationChangedNotice(ctx, g.ContactEmail, g.ContactFirstName, changes, true)
	// Stripe is the primary sender. Only if Stripe couldn't email the invoice
	// (e.g. restricted account) do we fall back to emailing the link ourselves.
	if !res.ProviderEmailed {
		emailCoachCount := int64(0)
		if coachIncluded {
			emailCoachCount = coachCount
		}
		if err := s.mailer.SendBalanceInvoice(ctx, notify.BalanceInvoice{
			ToEmail: g.ContactEmail,
			ToName:  g.ContactFirstName,
			PayURL:  res.HostedURL,
			DueAt:   dueAtRef(res.DueAt),
			Amount:  notifyAmount(res.AmountDuePence, res.Currency),
			Items:   notify.BalanceItems(campers, accNames, s.pricer.UnitNames(ctx), emailCoachCount),
			Changes: changes,
		}); err != nil {
			log.Printf("billing: fallback balance invoice email to %s: %v", g.ContactEmail, err)
		}
	}
	return nil
}

func (s *Service) SendInvoicesBulk(ctx context.Context, actor string, groupIDs []string) map[string]error {
	errs := map[string]error{}
	for _, id := range groupIDs {
		g, err := s.reads.FindGroupByID(ctx, id)
		if err != nil {
			errs[id] = err
			continue
		}
		if g != nil && g.SkipsBulkInvoice() {
			continue
		}
		if err := s.SendInvoice(ctx, id, actor, domain.SkipVersionCheck); err != nil {
			errs[id] = err
		}
	}
	return errs
}

func (s *Service) SendCoachInvoice(ctx context.Context, groupID, actor string, expectedVersion int) error {
	g, err := s.reads.FindGroupByID(ctx, groupID)
	if err != nil {
		return fmt.Errorf("operation: %w", err)
	}
	if g == nil {
		return domain.ErrGroupNotFound
	}
	if err := g.CanSendCoachInvoice(); err != nil {
		return err
	}
	campers, err := s.reads.CampersForGroup(ctx, groupID)
	if err != nil {
		return fmt.Errorf("operation: %w", err)
	}
	count := domain.CoachEligibleCount(campers)
	if err := g.CheckCoachInvoicePassengers(count); err != nil {
		return err
	}
	if s.cfg.CoachPriceRef == "" {
		return billing.ErrCoachPriceNotConfigured
	}

	customerID := ""
	if g.BillingCustomerRef != nil {
		customerID = *g.BillingCustomerRef
	}
	name := g.ContactName()
	customerID, err = s.invoices.EnsureCustomer(ctx, customerID, g.ContactEmail, name, g.ID)
	if err != nil {
		return fmt.Errorf("operation: %w", err)
	}
	if customerID != "" && (g.BillingCustomerRef == nil || *g.BillingCustomerRef != customerID) {
		if err := s.repo.SetBillingCustomerRef(ctx, groupID, customerID); err != nil {
			return fmt.Errorf("operation: %w", err)
		}
	}

	res, err := s.invoices.CreateInvoice(
		ctx, customerID, g.ID,
		[]billing.InvoiceLine{{PriceID: s.cfg.CoachPriceRef, Quantity: count}},
		int64(s.cfg.InvoiceDueDays), "coach")
	if err != nil {
		return fmt.Errorf("operation: %w", err)
	}
	meta := domain.ActionMeta{
		Actor:           actor,
		Action:          "coach_invoice_sent",
		ExpectedVersion: expectedVersion,
	}
	if err := s.repo.SetCoachInvoiceMeta(ctx, groupID, res.ID, res.DueAt, meta); err != nil {
		return billing.MapVersionErr(ctx, s.reads, groupID, err)
	}

	// Stripe is the primary sender; fall back to our own email only if Stripe
	// could not email the invoice.
	if !res.ProviderEmailed {
		if err := s.mailer.SendCoachInvoice(ctx, notify.CoachInvoice{
			ToEmail:        g.ContactEmail,
			ToName:         g.ContactFirstName,
			PayURL:         res.HostedURL,
			DueAt:          dueAtRef(res.DueAt),
			Amount:         notifyAmount(res.AmountDuePence, res.Currency),
			PassengerCount: int(count),
		}); err != nil {
			log.Printf("billing: fallback coach invoice email to %s: %v", g.ContactEmail, err)
		}
	}
	return nil
}

func (s *Service) SendCoachInvoicesBulk(ctx context.Context, actor string, groupIDs []string) map[string]error {
	errs := map[string]error{}
	for _, id := range groupIDs {
		if err := s.SendCoachInvoice(ctx, id, actor, domain.SkipVersionCheck); err != nil {
			errs[id] = err
		}
	}
	return errs
}

func (s *Service) WaiveCoachFee(ctx context.Context, groupID, actor string, expectedVersion int) error {
	g, err := s.reads.FindGroupByID(ctx, groupID)
	if err != nil {
		return fmt.Errorf("operation: %w", err)
	}
	if g == nil {
		return domain.ErrGroupNotFound
	}
	if err := g.CanWaiveCoachFee(); err != nil {
		return err
	}
	campers, err := s.reads.CampersForGroup(ctx, groupID)
	if err != nil {
		return fmt.Errorf("operation: %w", err)
	}
	if err := g.CheckCoachWaivePassengers(domain.CoachEligibleCount(campers)); err != nil {
		return err
	}
	if g.CoachInvoiceRef != nil && strings.TrimSpace(*g.CoachInvoiceRef) != "" {
		if err := s.invoices.VoidInvoiceIdempotent(ctx, *g.CoachInvoiceRef); err != nil {
			return apperr.Invalidf("coach invoice void failed; coach fee was not waived: %v", err)
		}
	}
	meta := domain.ActionMeta{
		Actor:           actor,
		Action:          "coach_fee_waived",
		ExpectedVersion: expectedVersion,
	}
	if err := s.repo.WaiveCoachFeeMeta(ctx, groupID, meta); err != nil {
		return billing.MapVersionErr(ctx, s.reads, groupID, err)
	}
	return nil
}

func (s *Service) UnwaiveCoachFee(ctx context.Context, groupID, actor string, expectedVersion int) error {
	g, err := s.reads.FindGroupByID(ctx, groupID)
	if err != nil {
		return fmt.Errorf("operation: %w", err)
	}
	if g == nil {
		return domain.ErrGroupNotFound
	}
	if err := g.CanUnwaiveCoachFee(); err != nil {
		return err
	}
	meta := domain.ActionMeta{
		Actor:           actor,
		Action:          "coach_fee_unwaived",
		ExpectedVersion: expectedVersion,
	}
	if err := s.repo.UnwaiveCoachFeeMeta(ctx, groupID, meta); err != nil {
		return billing.MapVersionErr(ctx, s.reads, groupID, err)
	}
	return nil
}

func (s *Service) MarkBalancePaidManually(ctx context.Context, groupID, actor string, expectedVersion int) error {
	g, err := s.reads.FindGroupByID(ctx, groupID)
	if err != nil {
		return fmt.Errorf("operation: %w", err)
	}
	if g == nil {
		return domain.ErrGroupNotFound
	}
	tr, err := g.MarkBalancePaidManually()
	if err != nil {
		return err
	}

	var paidAmount notify.Amount
	if g.BillingStatus == domain.BillingInvoiced &&
		g.BalanceInvoiceRef != nil && *g.BalanceInvoiceRef != "" {
		res, err := s.invoices.GetInvoice(ctx, *g.BalanceInvoiceRef)
		if err != nil {
			log.Printf("billing: read invoice %s before manual mark-paid: %v", *g.BalanceInvoiceRef, err)
		} else {
			paidAmount = notifyAmount(res.AmountDuePence, res.Currency)
		}
		if err := s.invoices.VoidInvoiceIdempotent(ctx, *g.BalanceInvoiceRef); err != nil {
			return apperr.Invalidf("invoice void failed; group was not marked paid: %v", err)
		}
	}

	meta := domain.ActionMeta{
		Actor:           actor,
		Action:          "balance_paid",
		ExpectedVersion: expectedVersion,
	}
	if err := s.repo.MarkBalancePaidMeta(ctx, groupID, meta, tr); err != nil {
		return billing.MapVersionErr(ctx, s.reads, groupID, err)
	}

	s.sendManualBalancePaidEmails(ctx, g, paidAmount)
	return nil
}

func (s *Service) sendManualBalancePaidEmails(ctx context.Context, g *domain.Group, amount notify.Amount) {
	if s.mailer == nil {
		return
	}
	campers, err := s.reads.CampersForGroup(ctx, g.ID)
	if err != nil {
		log.Printf("billing: manual mark-paid load campers (group %s): %v", g.ID, err)
		return
	}
	accNames := s.pricer.AccommodationNames(ctx)
	coachCount := int64(0)
	if g.CoachIncludedInBalance {
		coachCount = domain.CoachEligibleCount(campers)
	}
	items := notify.BalanceItems(campers, accNames, s.pricer.UnitNames(ctx), coachCount)

	if err := s.mailer.SendBalancePaidConfirmation(ctx, notify.BalancePaidConfirmation{
		ToEmail: g.ContactEmail,
		ToName:  g.ContactFirstName,
		Amount:  amount,
		Items:   items,
		Changes: notify.OffPreferenceChanges(campers, accNames),
	}); err != nil {
		log.Printf("billing: manual balance-paid confirmation to %s: %v", g.ContactEmail, err)
	}

	if s.cfg.WhiteTeamEmail != "" {
		_ = s.mailer.SendBalancePaid(ctx, notify.BalancePaid{
			ToEmail:      s.cfg.WhiteTeamEmail,
			ContactName:  g.ContactName(),
			ContactEmail: g.ContactEmail,
			Amount:       amount,
			PaidAt:       time.Now(),
			Items:        items,
		})
	}
}

func (s *Service) DeleteRegistration(ctx context.Context, groupID, actor string, expectedVersion int) (DeleteSummary, error) {
	g, err := s.reads.FindGroupByID(ctx, groupID)
	if err != nil {
		return DeleteSummary{}, fmt.Errorf("operation: %w", err)
	}
	if g == nil {
		return DeleteSummary{}, domain.ErrGroupNotFound
	}

	// A registration paid for in full at signup put the deposit and everything
	// else on one charge, and the deposit portion is stored nowhere, so there is
	// no way to refund the rest without either guessing or refunding money the
	// church is entitled to keep. Hand that decision to a human in Stripe.
	if err := g.CanDelete(); err != nil {
		return DeleteSummary{}, err
	}

	summary := DeleteSummary{
		ContactName:  g.ContactName(),
		ContactEmail: g.ContactEmail,
	}

	campers, err := s.reads.CampersForGroup(ctx, groupID)
	if err != nil {
		return DeleteSummary{}, fmt.Errorf("operation: %w", err)
	}
	var depositOnInvoice int64
	summary.RetainedPence, depositOnInvoice = domain.DeleteRetention(g, campers)

	var refundTotal int64

	if g.HasSettledBalanceInvoice() {
		amt, err := s.invoices.RefundInvoiceKeeping(ctx, *g.BalanceInvoiceRef, depositOnInvoice)
		if err != nil {
			return DeleteSummary{}, apperr.Invalidf("balance refund failed; registration was not deleted: %v", err)
		}
		summary.BalanceRefunded = amt > 0
		refundTotal += amt
	}

	if g.HasOpenInvoice() {
		if err := s.invoices.VoidInvoiceIdempotent(ctx, *g.BalanceInvoiceRef); err != nil {
			return DeleteSummary{}, apperr.Invalidf("invoice void failed; registration was not deleted: %v", err)
		}
		summary.InvoiceVoided = true
	}

	// Void any open (unpaid) separate coach invoice so it can't be paid after
	// the registration is gone. Paid coach fees are refunded manually in Stripe.
	if g.CoachInvoiceRef != nil && *g.CoachInvoiceRef != "" && g.CoachFeePaidAt == nil {
		if err := s.invoices.VoidInvoiceIdempotent(ctx, *g.CoachInvoiceRef); err != nil {
			return DeleteSummary{}, apperr.Invalidf("coach invoice void failed; registration was not deleted: %v", err)
		}
	}

	summary.AmountPence = refundTotal

	meta := domain.ActionMeta{
		Actor:           actor,
		Action:          "registration_deleted",
		ExpectedVersion: expectedVersion,
	}
	if err := s.repo.DeleteGroupMeta(ctx, groupID, meta); err != nil {
		return DeleteSummary{}, billing.MapVersionErr(ctx, s.reads, groupID, err)
	}

	if err := s.sheets.RemoveByGroupID(ctx, groupID); err != nil {
		log.Printf("billing: remove group %s from sheet: %v", groupID, err)
	}

	return summary, nil
}

func (s *Service) ResendInvoice(ctx context.Context, groupID, actor string, expectedVersion int) error {
	g, err := s.reads.FindGroupByID(ctx, groupID)
	if err != nil {
		return fmt.Errorf("operation: %w", err)
	}
	if g == nil {
		return domain.ErrGroupNotFound
	}
	openTR, err := g.CanResendInvoice()
	if err != nil {
		return err
	}
	// Primary: ask Stripe to re-notify. If it succeeds, stamp and return.
	if err := s.invoices.SendInvoiceEmail(ctx, *g.BalanceInvoiceRef); err == nil {
		meta := domain.ActionMeta{
			Actor:           actor,
			Action:          "invoice_resent",
			ExpectedVersion: expectedVersion,
		}
		if err := s.repo.StampOnly(ctx, groupID, meta, openTR); err != nil {
			return billing.MapVersionErr(ctx, s.reads, groupID, err)
		}
		return nil
	} else {
		log.Printf("billing: Stripe re-send failed for group %s; falling back to Resend: %v", groupID, err)
	}
	// Fallback: email the hosted payment link ourselves.
	res, err := s.invoices.GetInvoice(ctx, *g.BalanceInvoiceRef)
	if err != nil {
		return fmt.Errorf("operation: %w", err)
	}
	campers, _ := s.reads.CampersForGroup(ctx, groupID)
	coachCount := g.CoachCountOnBalance(campers)
	if err := s.mailer.SendBalanceInvoice(ctx, notify.BalanceInvoice{
		ToEmail: g.ContactEmail,
		ToName:  g.ContactFirstName,
		PayURL:  res.HostedURL,
		DueAt:   resendDueAt(g, res),
		Amount:  notifyAmount(res.AmountDuePence, res.Currency),
		Items:   notify.BalanceItems(campers, s.pricer.AccommodationNames(ctx), s.pricer.UnitNames(ctx), coachCount),
	}); err != nil {
		return fmt.Errorf("operation: %w", err)
	}
	meta := domain.ActionMeta{
		Actor:           actor,
		Action:          "invoice_resent",
		ExpectedVersion: expectedVersion,
	}
	if err := s.repo.StampOnly(ctx, groupID, meta, openTR); err != nil {
		return billing.MapVersionErr(ctx, s.reads, groupID, err)
	}
	return nil
}

func (s *Service) ExtendDueAt(ctx context.Context, groupID, actor string, expectedVersion int, dueAt time.Time) error {
	g, err := s.reads.FindGroupByID(ctx, groupID)
	if err != nil {
		return fmt.Errorf("operation: %w", err)
	}
	if g == nil {
		return domain.ErrGroupNotFound
	}
	tr, err := g.RequireOpenInvoice()
	if err != nil {
		return err
	}
	meta := domain.ActionMeta{
		Actor:           actor,
		Action:          "extend_due",
		ExpectedVersion: expectedVersion,
	}
	if err := s.repo.ExtendInvoiceDueAtMeta(ctx, groupID, dueAt, meta, tr); err != nil {
		return billing.MapVersionErr(ctx, s.reads, groupID, err)
	}
	return nil
}

func dayPassLine(c domain.Camper, priceID string) (billing.InvoiceLine, bool) {
	if c.AttendanceType != domain.AttendanceDayPass {
		return billing.InvoiceLine{}, false
	}
	if len(c.DayPassDays) == 0 {
		return billing.InvoiceLine{}, false
	}
	return billing.InvoiceLine{PriceID: priceID, Quantity: int64(len(c.DayPassDays))}, true
}

func coachLine(count int64, priceID string) (billing.InvoiceLine, bool) {
	if count <= 0 {
		return billing.InvoiceLine{}, false
	}
	return billing.InvoiceLine{PriceID: priceID, Quantity: count}, true
}

func (s *Service) sendAccommodationChangedNotice(ctx context.Context, toEmail, toName string, changes []notify.AccommodationChange, awaitingPayment bool) {
	if s.mailer == nil || len(changes) == 0 {
		return
	}
	if err := s.mailer.SendAccommodationChanged(ctx, notify.AccommodationChangedNotice{
		ToEmail:         toEmail,
		ToName:          toName,
		Items:           changes,
		AwaitingPayment: awaitingPayment,
	}); err != nil {
		log.Printf("billing: accommodation changed email to %s: %v", toEmail, err)
	}
}
