package invoicing

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"pagasacentre/backend/internal/billing"
	"pagasacentre/backend/internal/email"
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
			if c.AllocatedAccommodationCode == nil || strings.TrimSpace(*c.AllocatedAccommodationCode) == "" {
				return apperr.Invalidf("camper %s %s is not allocated", c.FirstName, c.LastName)
			}
			// Re-resolve the Stripe Price from the CURRENT accommodation config at
			// send time rather than replaying the value stored when the allocation
			// was first saved. This self-heals when Price IDs are corrected after
			// allocation (e.g. env vars set later), instead of erroring on a stale
			// "No such price".
			priceID, err := s.pricer.ResolvePriceID(ctx, strings.TrimSpace(*c.AllocatedAccommodationCode), c.Age)
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
	coachCount := billing.CoachEligibleCount(campers)
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
	name := strings.TrimSpace(g.ContactFirstName + " " + g.ContactLastName)
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
	changes := billing.OffPreferenceChanges(campers, accNames)
	s.sendAccommodationChangedNotice(ctx, g.ContactEmail, g.ContactFirstName, changes, true)
	// Stripe is the primary sender. Only if Stripe couldn't email the invoice
	// (e.g. restricted account) do we fall back to emailing the link ourselves.
	if !res.ProviderEmailed {
		emailCoachCount := int64(0)
		if coachIncluded {
			emailCoachCount = coachCount
		}
		if err := s.mailer.SendBalanceInvoice(ctx, email.BalanceInvoice{
			ToEmail:     g.ContactEmail,
			ToName:      g.ContactFirstName,
			PayURL:      res.HostedURL,
			DueDate:     res.DueAt.Format("2 Jan 2006"),
			AmountLabel: billing.FormatAmountLabel(res.AmountDuePence, res.Currency),
			Items:       billing.BalanceInvoiceItems(campers, accNames, s.pricer.UnitNames(ctx), emailCoachCount),
			Changes:     changes,
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
		if g != nil && g.IsFree {
			continue
		}
		if g != nil && g.PaidInFullAtRegistration {
			continue
		}
		if err := s.SendInvoice(ctx, id, actor, domain.SkipVersionCheck); err != nil {
			errs[id] = err
		}
	}
	return errs
}

func coachAlreadyCharged(g *domain.Group) bool {
	return g.CoachIncludedInBalance ||
		(g.CoachInvoiceRef != nil && strings.TrimSpace(*g.CoachInvoiceRef) != "")
}

func (s *Service) SendCoachInvoice(ctx context.Context, groupID, actor string, expectedVersion int) error {
	g, err := s.reads.FindGroupByID(ctx, groupID)
	if err != nil {
		return fmt.Errorf("operation: %w", err)
	}
	if g == nil {
		return domain.ErrGroupNotFound
	}
	if g.IsFree {
		return billing.ErrSponsoredNoCoach
	}
	if g.CoachFeeWaivedAt != nil {
		return apperr.Invalid("coach fee has been waived for this group")
	}
	if g.BillingStatus != domain.BillingInvoiced && g.BillingStatus != domain.BillingBalancePaid {
		return billing.ErrCoachInBalance
	}
	if coachAlreadyCharged(g) {
		return apperr.Invalid("coach fee already charged")
	}
	campers, err := s.reads.CampersForGroup(ctx, groupID)
	if err != nil {
		return fmt.Errorf("operation: %w", err)
	}
	count := billing.CoachEligibleCount(campers)
	if count == 0 {
		return apperr.Invalid("no coach passengers to charge")
	}
	if s.cfg.CoachPriceRef == "" {
		return billing.ErrCoachPriceNotConfigured
	}

	customerID := ""
	if g.BillingCustomerRef != nil {
		customerID = *g.BillingCustomerRef
	}
	name := strings.TrimSpace(g.ContactFirstName + " " + g.ContactLastName)
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
		if err := s.mailer.SendCoachInvoice(ctx, email.CoachInvoice{
			ToEmail:        g.ContactEmail,
			ToName:         g.ContactFirstName,
			PayURL:         res.HostedURL,
			DueDate:        res.DueAt.Format("2 Jan 2006"),
			AmountLabel:    billing.FormatAmountLabel(res.AmountDuePence, res.Currency),
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
	if g.IsFree {
		return billing.ErrSponsoredNoCoach
	}
	if g.CoachFeeWaivedAt != nil {
		return apperr.Invalid("coach fee already waived")
	}
	if g.CoachIncludedInBalance {
		return billing.ErrCoachInBalance
	}
	if g.CoachFeePaidAt != nil {
		return apperr.Invalid("coach fee already paid")
	}
	campers, err := s.reads.CampersForGroup(ctx, groupID)
	if err != nil {
		return fmt.Errorf("operation: %w", err)
	}
	if billing.CoachEligibleCount(campers) == 0 {
		return apperr.Invalid("no coach passengers to waive")
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
	if g.CoachFeeWaivedAt == nil {
		return apperr.Invalid("coach fee is not waived for this group")
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

	amountLabel := ""
	if g.BillingStatus == domain.BillingInvoiced &&
		g.BalanceInvoiceRef != nil && *g.BalanceInvoiceRef != "" {
		res, err := s.invoices.GetInvoice(ctx, *g.BalanceInvoiceRef)
		if err != nil {
			log.Printf("billing: read invoice %s before manual mark-paid: %v", *g.BalanceInvoiceRef, err)
		} else {
			amountLabel = billing.FormatAmountLabel(res.AmountDuePence, res.Currency)
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

	s.sendManualBalancePaidEmails(ctx, g, amountLabel)
	return nil
}

func (s *Service) sendManualBalancePaidEmails(ctx context.Context, g *domain.Group, amountLabel string) {
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
		coachCount = billing.CoachEligibleCount(campers)
	}
	items := billing.BalanceInvoiceItems(campers, accNames, s.pricer.UnitNames(ctx), coachCount)

	if err := s.mailer.SendBalancePaidConfirmation(ctx, email.BalancePaidConfirmation{
		ToEmail:     g.ContactEmail,
		ToName:      g.ContactFirstName,
		AmountLabel: amountLabel,
		Items:       items,
		Changes:     billing.OffPreferenceChanges(campers, accNames),
	}); err != nil {
		log.Printf("billing: manual balance-paid confirmation to %s: %v", g.ContactEmail, err)
	}

	if s.cfg.WhiteTeamEmail != "" {
		_ = s.mailer.SendBalancePaid(ctx, email.BalancePaid{
			ToEmail:      s.cfg.WhiteTeamEmail,
			ContactName:  strings.TrimSpace(g.ContactFirstName + " " + g.ContactLastName),
			ContactEmail: g.ContactEmail,
			AmountLabel:  amountLabel,
			PaidDate:     time.Now().Format("2 Jan 2006"),
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
	if g.PaidInFullAtRegistration && g.PaymentStatus == domain.PaymentPaid {
		return DeleteSummary{}, apperr.Invalid(
			"this family paid for everything in one payment, so the deposit cannot be " +
				"separated from the rest automatically. Refund what they are owed in " +
				"Stripe first, then delete the registration.")
	}

	summary := DeleteSummary{
		ContactName:  strings.TrimSpace(g.ContactFirstName + " " + g.ContactLastName),
		ContactEmail: g.ContactEmail,
	}

	// Deposit money arrives by two routes: the group's checkout at registration,
	// and a line on the balance invoice for anyone added after that checkout
	// settled. Both are kept, so both count towards what was retained, and the
	// invoiced part has to come off the balance refund below.
	campers, err := s.reads.CampersForGroup(ctx, groupID)
	if err != nil {
		return DeleteSummary{}, fmt.Errorf("operation: %w", err)
	}
	var depositOnInvoice int64
	for _, c := range campers {
		depositOnInvoice += int64(c.DepositPaidPence)
	}
	if g.PaymentStatus == domain.PaymentPaid {
		summary.RetainedPence = int64(g.TotalAmountPence)
	}
	summary.RetainedPence += depositOnInvoice

	var refundTotal int64

	if g.BalanceSettled() && g.BalanceInvoiceRef != nil && *g.BalanceInvoiceRef != "" {
		amt, err := s.invoices.RefundInvoiceKeeping(ctx, *g.BalanceInvoiceRef, depositOnInvoice)
		if err != nil {
			return DeleteSummary{}, apperr.Invalidf("balance refund failed; registration was not deleted: %v", err)
		}
		summary.BalanceRefunded = amt > 0
		refundTotal += amt
	}

	if g.BillingStatus == domain.BillingInvoiced && g.BalanceInvoiceRef != nil && *g.BalanceInvoiceRef != "" {
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
	if g.BalanceInvoiceRef == nil || *g.BalanceInvoiceRef == "" {
		return apperr.Invalid("no invoice on file")
	}
	openTR, err := g.RequireOpenInvoice()
	if err != nil {
		return err
	}
	// Primary: ask Stripe to re-email. If it succeeds, stamp and return.
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
	due := ""
	if g.InvoiceDueAt != nil {
		due = g.InvoiceDueAt.Format("2 Jan 2006")
	} else if !res.DueAt.IsZero() {
		due = res.DueAt.Format("2 Jan 2006")
	}
	campers, _ := s.reads.CampersForGroup(ctx, groupID)
	coachCount := int64(0)
	if g.CoachIncludedInBalance {
		coachCount = billing.CoachEligibleCount(campers)
	}
	if err := s.mailer.SendBalanceInvoice(ctx, email.BalanceInvoice{
		ToEmail:     g.ContactEmail,
		ToName:      g.ContactFirstName,
		PayURL:      res.HostedURL,
		DueDate:     due,
		AmountLabel: billing.FormatAmountLabel(res.AmountDuePence, res.Currency),
		Items:       billing.BalanceInvoiceItems(campers, s.pricer.AccommodationNames(ctx), s.pricer.UnitNames(ctx), coachCount),
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

func (s *Service) sendAccommodationChangedNotice(ctx context.Context, toEmail, toName string, changes []email.AccommodationChange, awaitingPayment bool) {
	if s.mailer == nil || len(changes) == 0 {
		return
	}
	if err := s.mailer.SendAccommodationChanged(ctx, email.AccommodationChangedNotice{
		ToEmail:         toEmail,
		ToName:          toName,
		Items:           changes,
		AwaitingPayment: awaitingPayment,
	}); err != nil {
		log.Printf("billing: accommodation changed email to %s: %v", toEmail, err)
	}
}
