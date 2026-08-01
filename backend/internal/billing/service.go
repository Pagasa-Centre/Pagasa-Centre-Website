package billing

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"pagasacentre/backend/internal/email"
	"pagasacentre/backend/internal/registration"
	"pagasacentre/backend/internal/sheets"
	commonerrors "pagasacentre/backend/pkg/commonlibrary/errors"
	"pagasacentre/backend/internal/registration/domain"
	"pagasacentre/backend/internal/registration/storage"
)

// stripeClient is the Stripe surface billing.Service needs. *StripeBilling satisfies it.
type stripeClient interface {
	EnsureCustomer(ctx context.Context, existingID, email, name, groupID string) (string, error)
	CreateInvoice(ctx context.Context, customerID, groupID string, lines []InvoiceLine, daysUntilDue int64, invoiceType string) (InvoiceResult, error)
	VoidInvoice(ctx context.Context, invoiceID string) error
	VoidInvoiceIdempotent(ctx context.Context, invoiceID string) error
	SendInvoiceEmail(ctx context.Context, invoiceID string) error
	GetInvoice(ctx context.Context, invoiceID string) (InvoiceResult, error)
	RefundPaymentIntent(ctx context.Context, paymentIntentID string) (int64, error)
	RefundInvoice(ctx context.Context, invoiceID string) (int64, error)
	CreditCustomerBalance(ctx context.Context, customerID string, amountPence int64, currency, description, idempotencyKey string) error
}

// Service coordinates allocation, Stripe Invoices, and release sweeps.
type Service struct {
	repo   *storage.Repository
	stripe stripeClient
	mailer email.Mailer
	sheets sheets.Sync
	cfg    Config
}

func NewService(repo *storage.Repository, stripe stripeClient, mailer email.Mailer, sheetSync sheets.Sync, cfg Config) *Service {
	if cfg.InvoiceDueDays <= 0 {
		cfg.InvoiceDueDays = 15
	}
	if sheetSync == nil {
		sheetSync = sheets.NewNoopSync()
	}
	return &Service{repo: repo, stripe: stripe, mailer: mailer, sheets: sheetSync, cfg: cfg}
}

// Allocate persists White Team placements and sets billing_status=allocated.
func (s *Service) Allocate(ctx context.Context, groupID, actor string, expectedVersion int, req AllocateRequest) error {
	g, err := s.repo.FindGroupByID(ctx, groupID)
	if err != nil {
		return commonerrors.Internal(err.Error())
	}
	if g == nil {
		return commonerrors.NotFound("group not found")
	}
	if g.PaymentStatus != domain.PaymentPaid {
		return commonerrors.BadRequest("deposit must be paid before allocation", nil)
	}
	if g.BillingStatus == domain.BillingInvoiced {
		return commonerrors.BadRequest("cannot change allocation while invoice is open; release first", nil)
	}
	if g.BillingStatus == domain.BillingBalancePaid && !g.PaidInFullAtRegistration {
		return commonerrors.BadRequest("balance already paid", nil)
	}

	campers, err := s.repo.CampersForGroup(ctx, groupID)
	if err != nil {
		return commonerrors.Internal(err.Error())
	}
	byID := map[string]domain.Camper{}
	for _, c := range campers {
		byID[c.ID] = c
	}

	var allocs []storage.CamperAllocation
	for _, a := range req.Campers {
		c, ok := byID[a.CamperID]
		if !ok {
			return commonerrors.BadRequest("unknown camper_id "+a.CamperID, nil)
		}
		if c.AttendanceType != domain.AttendanceFullWeek {
			continue
		}
		code := strings.TrimSpace(a.AllocatedAccommodationCode)
		if code == "" {
			return commonerrors.ValidationFailed(map[string]string{
				a.CamperID: "allocated_accommodation_code is required for full-week campers",
			})
		}
		priceID := strings.TrimSpace(a.BilledStripePriceID)
		if g.IsFree {
			priceID = ""
		} else if priceID == "" {
			priceID, err = s.resolvePriceID(ctx, code, c.Age)
			if err != nil {
				return commonerrors.BadRequest(err.Error(), nil)
			}
		}
		unitCode := strings.TrimSpace(a.AllocatedUnitCode)
		if err := s.validateAllocatedUnit(ctx, code, unitCode); err != nil {
			return err
		}
		allocs = append(allocs, storage.CamperAllocation{
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
			return commonerrors.ValidationFailed(map[string]string{
				"campers": fmt.Sprintf("full-week camper %s %s is not allocated", c.FirstName, c.LastName),
			})
		}
	}

	meta := domain.ActionMeta{
		Actor:           actor,
		Action:          "allocated",
		ExpectedVersion: expectedVersion,
	}
	if err := s.repo.AllocateGroup(ctx, groupID, allocs, meta); err != nil {
		return mapVersionErr(ctx, s.repo, groupID, err)
	}
	return nil
}

// Unallocate clears a group's placements and returns it to the unallocated
// (needs-accommodation) state. Only valid before an invoice has been sent —
// once invoiced, use VoidAndRelease so the open Stripe invoice is voided too.
func (s *Service) Unallocate(ctx context.Context, groupID, actor string, expectedVersion int) error {
	g, err := s.repo.FindGroupByID(ctx, groupID)
	if err != nil {
		return commonerrors.Internal(err.Error())
	}
	if g == nil {
		return commonerrors.NotFound("group not found")
	}
	switch g.BillingStatus {
	case domain.BillingInvoiced:
		return commonerrors.BadRequest("invoice is open; use release instead", nil)
	case domain.BillingBalancePaid:
		if !g.PaidInFullAtRegistration {
			return commonerrors.BadRequest("balance already paid", nil)
		}
	}
	meta := domain.ActionMeta{
		Actor:           actor,
		Action:          "unallocated",
		ExpectedVersion: expectedVersion,
	}
	if err := s.repo.UnallocateGroup(ctx, groupID, meta); err != nil {
		return mapVersionErr(ctx, s.repo, groupID, err)
	}
	return nil
}

// validateAllocatedUnit checks referential integrity between tier and unit.
// Blank unit is allowed (warn-only model). Mismatched or unknown units reject.
func (s *Service) validateAllocatedUnit(ctx context.Context, tierCode, unitCode string) error {
	if unitCode == "" {
		return nil
	}
	u, err := s.repo.GetAccommodationUnit(ctx, unitCode)
	if err != nil {
		return commonerrors.Internal(err.Error())
	}
	if u == nil {
		return commonerrors.BadRequest("unknown unit "+unitCode, nil)
	}
	if u.AccommodationCode != tierCode {
		return commonerrors.BadRequest(
			fmt.Sprintf("unit %q belongs to %q, not %q", unitCode, u.AccommodationCode, tierCode),
			nil,
		)
	}
	return nil
}

func (s *Service) resolvePriceID(ctx context.Context, accommodationCode string, age int) (string, error) {
	t, err := s.repo.GetAccommodationType(ctx, accommodationCode)
	if err != nil {
		return "", err
	}
	return registration.AccommodationPriceID(accommodationCode, age, t, s.cfg.StripePriceChildUnder3)
}

// SendInvoice creates and emails a Stripe Invoice for an allocated group.
func (s *Service) SendInvoice(ctx context.Context, groupID, actor string, expectedVersion int) error {
	g, err := s.repo.FindGroupByID(ctx, groupID)
	if err != nil {
		return commonerrors.Internal(err.Error())
	}
	if g == nil {
		return commonerrors.NotFound("group not found")
	}
	if g.IsFree {
		return commonerrors.BadRequest("this is a church-sponsored registration; confirm the sponsorship instead of invoicing", nil)
	}
	if g.PaidInFullAtRegistration {
		return commonerrors.BadRequest("this family paid in full at registration; there is nothing to invoice", nil)
	}
	if g.PaymentStatus != domain.PaymentPaid {
		return commonerrors.BadRequest("deposit must be paid first", nil)
	}
	campers, err := s.repo.CampersForGroup(ctx, groupID)
	if err != nil {
		return commonerrors.Internal(err.Error())
	}
	hasFullWeek := false
	for _, c := range campers {
		if c.AttendanceType == domain.AttendanceFullWeek {
			hasFullWeek = true
			break
		}
	}

	switch g.BillingStatus {
	case domain.BillingBalancePaid:
		return commonerrors.BadRequest("balance already paid", nil)
	case domain.BillingInvoiced:
		return commonerrors.BadRequest("invoice already sent", nil)
	case domain.BillingNone, domain.BillingReleased:
		// Full-week campers must be allocated before invoicing. Day-pass-only
		// groups have no accommodation to allocate, so they invoice straight
		// from "none".
		if hasFullWeek {
			return commonerrors.BadRequest("group must be allocated before invoicing", nil)
		}
	}

	var lines []InvoiceLine
	for _, c := range campers {
		switch c.AttendanceType {
		case domain.AttendanceFullWeek:
			if c.AllocatedAccommodationCode == nil || strings.TrimSpace(*c.AllocatedAccommodationCode) == "" {
				return commonerrors.BadRequest(fmt.Sprintf("camper %s %s is not allocated", c.FirstName, c.LastName), nil)
			}
			// Re-resolve the Stripe Price from the CURRENT accommodation config at
			// send time rather than replaying the value stored when the allocation
			// was first saved. This self-heals when Price IDs are corrected after
			// allocation (e.g. env vars set later), instead of erroring on a stale
			// "No such price".
			priceID, err := s.resolvePriceID(ctx, strings.TrimSpace(*c.AllocatedAccommodationCode), c.Age)
			if err != nil {
				return commonerrors.BadRequest(err.Error(), nil)
			}
			lines = append(lines, InvoiceLine{PriceID: priceID, Quantity: 1})
		case domain.AttendanceDayPass:
			line, ok := dayPassLine(c, s.cfg.StripePriceDayPass)
			if !ok {
				continue
			}
			if line.PriceID == "" {
				return commonerrors.BadRequest("STRIPE_PRICE_DAY_PASS is not configured", nil)
			}
			lines = append(lines, line)
		}
	}

	// Fold the coach fee into this balance invoice for coach passengers. Only
	// groups that have not yet been invoiced reach here, so a coach fee that is
	// folded is always fresh (never double-charged). Skip when admin has waived.
	coachCount := coachEligibleCount(campers)
	coachIncluded := false
	if g.CoachFeeWaivedAt == nil {
		if line, ok := coachLine(coachCount, s.cfg.StripePriceCoach); ok {
			if line.PriceID == "" {
				return commonerrors.BadRequest("STRIPE_PRICE_COACH is not configured", nil)
			}
			lines = append(lines, line)
			coachIncluded = coachCount > 0
		}
	}

	if len(lines) == 0 {
		return commonerrors.BadRequest("nothing to invoice", nil)
	}

	customerID := ""
	if g.StripeCustomerID != nil {
		customerID = *g.StripeCustomerID
	}
	name := strings.TrimSpace(g.ContactFirstName + " " + g.ContactLastName)
	customerID, err = s.stripe.EnsureCustomer(ctx, customerID, g.ContactEmail, name, g.ID)
	if err != nil {
		return commonerrors.Internal(err.Error())
	}
	if customerID != "" && (g.StripeCustomerID == nil || *g.StripeCustomerID != customerID) {
		if err := s.repo.SetStripeCustomerID(ctx, groupID, customerID); err != nil {
			return commonerrors.Internal(err.Error())
		}
	}

	res, err := s.stripe.CreateInvoice(
		ctx, customerID, g.ID, lines, int64(s.cfg.InvoiceDueDays), "balance")
	if err != nil {
		return commonerrors.Internal(err.Error())
	}
	meta := domain.ActionMeta{
		Actor:           actor,
		Action:          "invoice_sent",
		ExpectedVersion: expectedVersion,
	}
	if err := s.repo.SetInvoiceDetailsMeta(ctx, groupID, res.ID, res.DueAt, coachIncluded, meta); err != nil {
		return mapVersionErr(ctx, s.repo, groupID, err)
	}
	accNames := s.accommodationNames(ctx)
	changes := offPreferenceChanges(campers, accNames)
	s.sendAccommodationChangedNotice(ctx, g.ContactEmail, g.ContactFirstName, changes, true)
	// Stripe is the primary sender. Only if Stripe couldn't email the invoice
	// (e.g. restricted account) do we fall back to emailing the link ourselves.
	if !res.StripeEmailed {
		emailCoachCount := int64(0)
		if coachIncluded {
			emailCoachCount = coachCount
		}
		if err := s.mailer.SendBalanceInvoice(ctx, email.BalanceInvoice{
			ToEmail:     g.ContactEmail,
			ToName:      g.ContactFirstName,
			PayURL:      res.HostedURL,
			DueDate:     res.DueAt.Format("2 Jan 2006"),
			AmountLabel: formatAmountLabel(res.AmountDuePence, res.Currency),
			Items:       balanceInvoiceItems(campers, accNames, s.unitNames(ctx), emailCoachCount),
			Changes:     changes,
		}); err != nil {
			log.Printf("billing: fallback balance invoice email to %s: %v", g.ContactEmail, err)
		}
	}
	return nil
}

// SendInvoicesBulk sends invoices for many groups; collects per-group errors.
func (s *Service) SendInvoicesBulk(ctx context.Context, actor string, groupIDs []string) map[string]string {
	errs := map[string]string{}
	for _, id := range groupIDs {
		g, err := s.repo.FindGroupByID(ctx, id)
		if err != nil {
			errs[id] = err.Error()
			continue
		}
		if g != nil && g.IsFree {
			continue
		}
		if g != nil && g.PaidInFullAtRegistration {
			continue
		}
		if err := s.SendInvoice(ctx, id, actor, domain.SkipVersionCheck); err != nil {
			errs[id] = err.Error()
		}
	}
	return errs
}

// coachAlreadyCharged reports whether the coach fee for this group has already
// been billed — either folded into the balance invoice or via a separate coach
// invoice.
func coachAlreadyCharged(g *domain.Group) bool {
	return g.CoachIncludedInBalance ||
		(g.StripeCoachInvoiceID != nil && strings.TrimSpace(*g.StripeCoachInvoiceID) != "")
}

// SendCoachInvoice issues a separate coach-only Stripe invoice for a group whose
// balance invoice was already sent or paid (so the coach fee could not be folded
// in). New/uninvoiced groups fold the coach fee into their balance invoice
// instead, and church-sponsored groups are never coach-charged.
func (s *Service) SendCoachInvoice(ctx context.Context, groupID, actor string, expectedVersion int) error {
	g, err := s.repo.FindGroupByID(ctx, groupID)
	if err != nil {
		return commonerrors.Internal(err.Error())
	}
	if g == nil {
		return commonerrors.NotFound("group not found")
	}
	if g.IsFree {
		return commonerrors.BadRequest("church-sponsored registrations are not charged for the coach", nil)
	}
	if g.CoachFeeWaivedAt != nil {
		return commonerrors.BadRequest("coach fee has been waived for this group", nil)
	}
	if g.BillingStatus != domain.BillingInvoiced && g.BillingStatus != domain.BillingBalancePaid {
		return commonerrors.BadRequest("coach fee is included in the balance invoice for this group", nil)
	}
	if coachAlreadyCharged(g) {
		return commonerrors.BadRequest("coach fee already charged", nil)
	}
	campers, err := s.repo.CampersForGroup(ctx, groupID)
	if err != nil {
		return commonerrors.Internal(err.Error())
	}
	count := coachEligibleCount(campers)
	if count == 0 {
		return commonerrors.BadRequest("no coach passengers to charge", nil)
	}
	if s.cfg.StripePriceCoach == "" {
		return commonerrors.BadRequest("STRIPE_PRICE_COACH is not configured", nil)
	}

	customerID := ""
	if g.StripeCustomerID != nil {
		customerID = *g.StripeCustomerID
	}
	name := strings.TrimSpace(g.ContactFirstName + " " + g.ContactLastName)
	customerID, err = s.stripe.EnsureCustomer(ctx, customerID, g.ContactEmail, name, g.ID)
	if err != nil {
		return commonerrors.Internal(err.Error())
	}
	if customerID != "" && (g.StripeCustomerID == nil || *g.StripeCustomerID != customerID) {
		if err := s.repo.SetStripeCustomerID(ctx, groupID, customerID); err != nil {
			return commonerrors.Internal(err.Error())
		}
	}

	res, err := s.stripe.CreateInvoice(
		ctx, customerID, g.ID,
		[]InvoiceLine{{PriceID: s.cfg.StripePriceCoach, Quantity: count}},
		int64(s.cfg.InvoiceDueDays), "coach")
	if err != nil {
		return commonerrors.Internal(err.Error())
	}
	meta := domain.ActionMeta{
		Actor:           actor,
		Action:          "coach_invoice_sent",
		ExpectedVersion: expectedVersion,
	}
	if err := s.repo.SetCoachInvoiceMeta(ctx, groupID, res.ID, res.DueAt, meta); err != nil {
		return mapVersionErr(ctx, s.repo, groupID, err)
	}

	// Stripe is the primary sender; fall back to our own email only if Stripe
	// could not email the invoice.
	if !res.StripeEmailed {
		if err := s.mailer.SendCoachInvoice(ctx, email.CoachInvoice{
			ToEmail:        g.ContactEmail,
			ToName:         g.ContactFirstName,
			PayURL:         res.HostedURL,
			DueDate:        res.DueAt.Format("2 Jan 2006"),
			AmountLabel:    formatAmountLabel(res.AmountDuePence, res.Currency),
			PassengerCount: int(count),
		}); err != nil {
			log.Printf("billing: fallback coach invoice email to %s: %v", g.ContactEmail, err)
		}
	}
	return nil
}

// SendCoachInvoicesBulk sends coach invoices for many groups; collects per-group
// errors (skips groups that don't qualify without erroring).
func (s *Service) SendCoachInvoicesBulk(ctx context.Context, actor string, groupIDs []string) map[string]string {
	errs := map[string]string{}
	for _, id := range groupIDs {
		if err := s.SendCoachInvoice(ctx, id, actor, domain.SkipVersionCheck); err != nil {
			errs[id] = err.Error()
		}
	}
	return errs
}

// WaiveCoachFee marks a group's coach fee as waived (paid separately). Voids an
// open unpaid coach invoice if one exists. Allowed before or after balance invoicing
// as long as the coach fee was not folded into a sent balance invoice.
func (s *Service) WaiveCoachFee(ctx context.Context, groupID, actor string, expectedVersion int) error {
	g, err := s.repo.FindGroupByID(ctx, groupID)
	if err != nil {
		return commonerrors.Internal(err.Error())
	}
	if g == nil {
		return commonerrors.NotFound("group not found")
	}
	if g.IsFree {
		return commonerrors.BadRequest("church-sponsored registrations are not charged for the coach", nil)
	}
	if g.CoachFeeWaivedAt != nil {
		return commonerrors.BadRequest("coach fee already waived", nil)
	}
	if g.CoachIncludedInBalance {
		return commonerrors.BadRequest("coach fee is included in the balance invoice for this group", nil)
	}
	if g.CoachFeePaidAt != nil {
		return commonerrors.BadRequest("coach fee already paid", nil)
	}
	campers, err := s.repo.CampersForGroup(ctx, groupID)
	if err != nil {
		return commonerrors.Internal(err.Error())
	}
	if coachEligibleCount(campers) == 0 {
		return commonerrors.BadRequest("no coach passengers to waive", nil)
	}
	if g.StripeCoachInvoiceID != nil && strings.TrimSpace(*g.StripeCoachInvoiceID) != "" {
		if err := s.stripe.VoidInvoiceIdempotent(ctx, *g.StripeCoachInvoiceID); err != nil {
			return commonerrors.BadRequest(
				fmt.Sprintf("coach invoice void failed; coach fee was not waived: %v", err), nil)
		}
	}
	meta := domain.ActionMeta{
		Actor:           actor,
		Action:          "coach_fee_waived",
		ExpectedVersion: expectedVersion,
	}
	if err := s.repo.WaiveCoachFeeMeta(ctx, groupID, meta); err != nil {
		return mapVersionErr(ctx, s.repo, groupID, err)
	}
	return nil
}

// UnwaiveCoachFee clears a prior coach-fee waiver so the group can be charged again.
func (s *Service) UnwaiveCoachFee(ctx context.Context, groupID, actor string, expectedVersion int) error {
	g, err := s.repo.FindGroupByID(ctx, groupID)
	if err != nil {
		return commonerrors.Internal(err.Error())
	}
	if g == nil {
		return commonerrors.NotFound("group not found")
	}
	if g.CoachFeeWaivedAt == nil {
		return commonerrors.BadRequest("coach fee is not waived for this group", nil)
	}
	meta := domain.ActionMeta{
		Actor:           actor,
		Action:          "coach_fee_unwaived",
		ExpectedVersion: expectedVersion,
	}
	if err := s.repo.UnwaiveCoachFeeMeta(ctx, groupID, meta); err != nil {
		return mapVersionErr(ctx, s.repo, groupID, err)
	}
	return nil
}

// HandleCoachInvoicePaid marks a group's coach fee paid (idempotent).
func (s *Service) HandleCoachInvoicePaid(ctx context.Context, stripeInvoiceID, groupID string) error {
	g, err := s.repo.FindGroupByID(ctx, groupID)
	if err != nil {
		return err
	}
	if g == nil {
		g, err = s.repo.FindGroupByStripeCoachInvoiceID(ctx, stripeInvoiceID)
		if err != nil || g == nil {
			return fmt.Errorf("group not found for coach invoice %s", stripeInvoiceID)
		}
	}
	if g.CoachFeePaidAt != nil {
		return nil
	}
	return s.repo.MarkCoachFeePaidMeta(ctx, g.ID, domain.ActionMeta{
		Actor:           "Stripe",
		Action:          "coach_fee_paid",
		ExpectedVersion: domain.SkipVersionCheck,
	})
}

// ConfirmFree marks a church-sponsored group as fully confirmed (no Stripe invoice).
func (s *Service) ConfirmFree(ctx context.Context, groupID, actor string, expectedVersion int) error {
	g, err := s.repo.FindGroupByID(ctx, groupID)
	if err != nil {
		return commonerrors.Internal(err.Error())
	}
	if g == nil {
		return commonerrors.NotFound("group not found")
	}
	if !g.IsFree {
		return commonerrors.BadRequest("this group is not church-sponsored", nil)
	}
	if g.PaymentStatus != domain.PaymentPaid {
		return commonerrors.BadRequest("deposit must be paid first", nil)
	}
	if g.BillingStatus != domain.BillingAllocated {
		return commonerrors.BadRequest("group must be allocated before confirming sponsorship", nil)
	}
	meta := domain.ActionMeta{
		Actor:           actor,
		Action:          "free_confirmed",
		ExpectedVersion: expectedVersion,
	}
	if err := s.repo.ConfirmFreeMeta(ctx, groupID, meta); err != nil {
		return mapVersionErr(ctx, s.repo, groupID, err)
	}
	s.sendSponsorshipConfirmedEmail(ctx, g)
	return nil
}

// sendSponsorshipConfirmedEmail tells the sponsored family their accommodation
// is confirmed. Best-effort: a failure here must not fail the confirmation,
// which has already been committed.
func (s *Service) sendSponsorshipConfirmedEmail(ctx context.Context, g *domain.Group) {
	if s.mailer == nil {
		return
	}
	campers, err := s.repo.CampersForGroup(ctx, g.ID)
	if err != nil {
		log.Printf("billing: sponsorship email load campers (group %s): %v", g.ID, err)
		return
	}
	accNames := s.accommodationNames(ctx)
	if err := s.mailer.SendSponsorshipConfirmed(ctx, email.SponsorshipConfirmed{
		ToEmail: g.ContactEmail,
		ToName:  g.ContactFirstName,
		Items:   balanceInvoiceItems(campers, accNames, s.unitNames(ctx), 0),
		Changes: offPreferenceChanges(campers, accNames),
	}); err != nil {
		log.Printf("billing: sponsorship confirmed email to %s: %v", g.ContactEmail, err)
	}
}

// MarkBalancePaidManually records a balance settled outside Stripe, such as a
// bank transfer. Any open invoice is voided first so the family cannot also pay
// online; if that void fails the group is left untouched rather than showing as
// paid with a live payment link.
func (s *Service) MarkBalancePaidManually(ctx context.Context, groupID, actor string, expectedVersion int) error {
	g, err := s.repo.FindGroupByID(ctx, groupID)
	if err != nil {
		return commonerrors.Internal(err.Error())
	}
	if g == nil {
		return commonerrors.NotFound("group not found")
	}
	if g.PaymentStatus != domain.PaymentPaid {
		return commonerrors.BadRequest("deposit must be paid first", nil)
	}
	if g.IsFree {
		return commonerrors.BadRequest(
			"church-sponsored groups are settled with Confirm sponsorship", nil)
	}
	switch g.BillingStatus {
	case domain.BillingAllocated, domain.BillingInvoiced:
	default:
		return commonerrors.BadRequest(
			"only allocated or invoiced groups can be marked paid; this group is "+
				billingStatusLabel(g.BillingStatus), nil)
	}

	amountLabel := ""
	if g.BillingStatus == domain.BillingInvoiced &&
		g.StripeInvoiceID != nil && *g.StripeInvoiceID != "" {
		res, err := s.stripe.GetInvoice(ctx, *g.StripeInvoiceID)
		if err != nil {
			log.Printf("billing: read invoice %s before manual mark-paid: %v", *g.StripeInvoiceID, err)
		} else {
			amountLabel = formatAmountLabel(res.AmountDuePence, res.Currency)
		}
		if err := s.stripe.VoidInvoiceIdempotent(ctx, *g.StripeInvoiceID); err != nil {
			return commonerrors.BadRequest(
				fmt.Sprintf("invoice void failed; group was not marked paid: %v", err), nil)
		}
	}

	meta := domain.ActionMeta{
		Actor:           actor,
		Action:          "balance_paid",
		ExpectedVersion: expectedVersion,
	}
	if err := s.repo.MarkBalancePaidMeta(ctx, groupID, meta); err != nil {
		return mapVersionErr(ctx, s.repo, groupID, err)
	}

	s.sendManualBalancePaidEmails(ctx, g, amountLabel)
	return nil
}

// sendManualBalancePaidEmails mirrors the Stripe balance-paid notifications for
// a payment recorded by hand. Best-effort: the payment is already committed, so
// a mail failure must not fail the action.
func (s *Service) sendManualBalancePaidEmails(ctx context.Context, g *domain.Group, amountLabel string) {
	if s.mailer == nil {
		return
	}
	campers, err := s.repo.CampersForGroup(ctx, g.ID)
	if err != nil {
		log.Printf("billing: manual mark-paid load campers (group %s): %v", g.ID, err)
		return
	}
	accNames := s.accommodationNames(ctx)
	coachCount := int64(0)
	if g.CoachIncludedInBalance {
		coachCount = coachEligibleCount(campers)
	}
	items := balanceInvoiceItems(campers, accNames, s.unitNames(ctx), coachCount)

	if err := s.mailer.SendBalancePaidConfirmation(ctx, email.BalancePaidConfirmation{
		ToEmail:     g.ContactEmail,
		ToName:      g.ContactFirstName,
		AmountLabel: amountLabel,
		Items:       items,
		Changes:     offPreferenceChanges(campers, accNames),
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

// VoidAndRelease voids the Stripe invoice (if any) and clears allocations.
func (s *Service) VoidAndRelease(ctx context.Context, groupID, reason, actor string, expectedVersion int, cancelled bool) error {
	g, err := s.repo.FindGroupByID(ctx, groupID)
	if err != nil {
		return commonerrors.Internal(err.Error())
	}
	if g == nil {
		return commonerrors.NotFound("group not found")
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
	meta := domain.ActionMeta{
		Actor:           actor,
		Action:          action,
		ExpectedVersion: expectedVersion,
	}
	if err := s.repo.ClearInvoiceAndReleaseMeta(ctx, groupID, meta); err != nil {
		return mapVersionErr(ctx, s.repo, groupID, err)
	}

	campers, _ := s.repo.CampersForGroup(ctx, groupID)
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

// RemoveCamperSummary captures the outcome of removing one camper from a group.
type RemoveCamperSummary struct {
	CamperName    string
	InvoiceVoided bool
}

// RemoveCamper hard-deletes one camper from a multi-person booking. No deposit
// refund is issued. Blocks removal of the main contact, the last camper, or
// anyone after the balance is paid. Voids an open invoice when present.
func (s *Service) RemoveCamper(ctx context.Context, groupID, camperID, actor string, expectedVersion int) (RemoveCamperSummary, error) {
	g, err := s.repo.FindGroupByID(ctx, groupID)
	if err != nil {
		return RemoveCamperSummary{}, commonerrors.Internal(err.Error())
	}
	if g == nil {
		return RemoveCamperSummary{}, commonerrors.NotFound("group not found")
	}

	campers, err := s.repo.CampersForGroup(ctx, groupID)
	if err != nil {
		return RemoveCamperSummary{}, commonerrors.Internal(err.Error())
	}

	var target *domain.Camper
	for i := range campers {
		if campers[i].ID == camperID {
			target = &campers[i]
			break
		}
	}
	if target == nil {
		return RemoveCamperSummary{}, commonerrors.NotFound("camper not found")
	}

	summary := RemoveCamperSummary{
		CamperName: strings.TrimSpace(target.FirstName + " " + target.LastName),
	}

	if target.IsMainContact {
		return RemoveCamperSummary{}, commonerrors.BadRequest("can't remove the main contact; edit the booking another way", nil)
	}
	if len(campers) <= 1 {
		return RemoveCamperSummary{}, commonerrors.BadRequest("this is the last camper; delete the whole registration instead", nil)
	}
	if g.BillingStatus == domain.BillingBalancePaid {
		return RemoveCamperSummary{}, commonerrors.BadRequest("balance already paid; refund/handle in Stripe first", nil)
	}

	newStatus := ""
	if g.BillingStatus == domain.BillingInvoiced && g.StripeInvoiceID != nil && *g.StripeInvoiceID != "" {
		if err := s.stripe.VoidInvoiceIdempotent(ctx, *g.StripeInvoiceID); err != nil {
			return RemoveCamperSummary{}, commonerrors.BadRequest(
				fmt.Sprintf("invoice void failed; camper was not removed: %v", err), nil)
		}
		summary.InvoiceVoided = true
		newStatus = domain.BillingNone
		for _, c := range campers {
			if c.ID == camperID {
				continue
			}
			if c.AttendanceType == domain.AttendanceFullWeek {
				newStatus = domain.BillingAllocated
				break
			}
		}
	}

	// If removing this camper leaves no coach passengers, void any open (unpaid)
	// separate coach invoice so a departed passenger's fee can't still be paid.
	if g.StripeCoachInvoiceID != nil && strings.TrimSpace(*g.StripeCoachInvoiceID) != "" && g.CoachFeePaidAt == nil {
		remainingCoach := int64(0)
		for _, c := range campers {
			if c.ID == camperID {
				continue
			}
			if c.AttendanceType == domain.AttendanceFullWeek && c.NeedsCoach != nil && *c.NeedsCoach && c.Age >= registration.MinDepositAge {
				remainingCoach++
			}
		}
		if remainingCoach == 0 {
			if err := s.stripe.VoidInvoiceIdempotent(ctx, *g.StripeCoachInvoiceID); err != nil {
				log.Printf("billing: void coach invoice after camper remove %s: %v", groupID, err)
			}
		}
	}

	meta := domain.ActionMeta{
		Actor:           actor,
		Action:          "camper_removed",
		ExpectedVersion: expectedVersion,
	}
	if err := s.repo.DeleteCamperMeta(ctx, groupID, camperID, newStatus, meta); err != nil {
		return RemoveCamperSummary{}, mapVersionErr(ctx, s.repo, groupID, err)
	}

	if g.PaymentStatus == domain.PaymentPaid {
		remaining, err := s.repo.CampersForGroup(ctx, groupID)
		if err != nil {
			log.Printf("billing: load campers after remove %s/%s: %v", groupID, camperID, err)
		} else {
			gAfter, err := s.repo.FindGroupByID(ctx, groupID)
			if err != nil || gAfter == nil {
				log.Printf("billing: reload group after remove %s: %v", groupID, err)
			} else {
				if err := s.sheets.RemoveByGroupID(ctx, groupID); err != nil {
					log.Printf("billing: remove group %s from sheet after camper remove: %v", groupID, err)
				}
				rows := sheetRowsForGroup(gAfter, remaining)
				if err := s.sheets.AppendPaidAndRemovePending(ctx, groupID, rows); err != nil {
					log.Printf("billing: re-sync sheet for group %s after camper remove: %v", groupID, err)
				}
			}
		}
	}

	return summary, nil
}

// ConvertSummary captures the outcome of converting a full-week camper to day-visitor.
type ConvertSummary struct {
	CamperName       string
	InvoiceVoided    bool
	DepositCreditPence int
}

// ConvertCamperToDayVisitor rewrites one full-week camper as a day-visitor in place.
// A Stripe customer-balance credit is applied immediately when the deposit was paid.
func (s *Service) ConvertCamperToDayVisitor(
	ctx context.Context,
	groupID, camperID, actor string,
	expectedVersion int,
	req ConvertToDayVisitorRequest,
) (ConvertSummary, error) {
	g, err := s.repo.FindGroupByID(ctx, groupID)
	if err != nil {
		return ConvertSummary{}, commonerrors.Internal(err.Error())
	}
	if g == nil {
		return ConvertSummary{}, commonerrors.NotFound("group not found")
	}

	campers, err := s.repo.CampersForGroup(ctx, groupID)
	if err != nil {
		return ConvertSummary{}, commonerrors.Internal(err.Error())
	}

	var target *domain.Camper
	for i := range campers {
		if campers[i].ID == camperID {
			target = &campers[i]
			break
		}
	}
	if target == nil {
		return ConvertSummary{}, commonerrors.NotFound("camper not found")
	}

	summary := ConvertSummary{
		CamperName: strings.TrimSpace(target.FirstName + " " + target.LastName),
	}

	if target.AttendanceType != domain.AttendanceFullWeek {
		return ConvertSummary{}, commonerrors.BadRequest("only full-week campers can be converted", nil)
	}
	if g.IsFree {
		return ConvertSummary{}, commonerrors.BadRequest(
			"church-sponsored registrations can't be converted here; handle manually", nil)
	}
	if g.BillingStatus == domain.BillingBalancePaid {
		return ConvertSummary{}, commonerrors.BadRequest("balance already paid; handle in Stripe first", nil)
	}

	fields := map[string]string{}
	registration.ValidateDayPass("attendance", domain.AttendanceDTO{
		Type:          domain.AttendanceDayPass,
		Days:          req.Days,
		TshirtOption:  req.TshirtOption,
		ShirtSize:     req.ShirtSize,
		NeedsCatering: req.NeedsCatering,
	}, fields)
	if len(fields) > 0 {
		return ConvertSummary{}, commonerrors.ValidationFailed(fields)
	}

	depositCreditPence := 0
	if g.PaymentStatus == domain.PaymentPaid && target.Age >= registration.MinDepositAge {
		depositCreditPence = s.cfg.DepositPricePence
	}
	summary.DepositCreditPence = depositCreditPence

	if depositCreditPence > 0 {
		customerID := ""
		if g.StripeCustomerID != nil {
			customerID = *g.StripeCustomerID
		}
		name := strings.TrimSpace(g.ContactFirstName + " " + g.ContactLastName)
		customerID, err = s.stripe.EnsureCustomer(ctx, customerID, g.ContactEmail, name, g.ID)
		if err != nil {
			return ConvertSummary{}, commonerrors.Internal(err.Error())
		}
		if customerID != "" && (g.StripeCustomerID == nil || *g.StripeCustomerID != customerID) {
			if err := s.repo.SetStripeCustomerID(ctx, groupID, customerID); err != nil {
				return ConvertSummary{}, commonerrors.Internal(err.Error())
			}
		}
		if err := s.stripe.CreditCustomerBalance(
			ctx, customerID, int64(depositCreditPence), g.Currency,
			"Deposit credit (converted to day visitor)", "deposit-credit-"+camperID,
		); err != nil {
			return ConvertSummary{}, commonerrors.Internal(err.Error())
		}
	}

	newStatus := ""
	if g.BillingStatus == domain.BillingInvoiced && g.StripeInvoiceID != nil && *g.StripeInvoiceID != "" {
		if err := s.stripe.VoidInvoiceIdempotent(ctx, *g.StripeInvoiceID); err != nil {
			return ConvertSummary{}, commonerrors.BadRequest(
				fmt.Sprintf("invoice void failed; camper was not converted: %v", err), nil)
		}
		summary.InvoiceVoided = true
		newStatus = domain.BillingNone
		for _, c := range campers {
			if c.ID == camperID {
				continue
			}
			if c.AttendanceType == domain.AttendanceFullWeek {
				newStatus = domain.BillingAllocated
				break
			}
		}
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

	dp := storage.DayPassFields{
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
	if err := s.repo.ConvertCamperToDayPassMeta(ctx, groupID, camperID, dp, depositCreditPence, newStatus, meta); err != nil {
		return ConvertSummary{}, mapVersionErr(ctx, s.repo, groupID, err)
	}

	if g.PaymentStatus == domain.PaymentPaid {
		remaining, err := s.repo.CampersForGroup(ctx, groupID)
		if err != nil {
			log.Printf("billing: load campers after convert %s/%s: %v", groupID, camperID, err)
		} else {
			gAfter, err := s.repo.FindGroupByID(ctx, groupID)
			if err != nil || gAfter == nil {
				log.Printf("billing: reload group after convert %s: %v", groupID, err)
			} else {
				if err := s.sheets.RemoveByGroupID(ctx, groupID); err != nil {
					log.Printf("billing: remove group %s from sheet after camper convert: %v", groupID, err)
				}
				rows := sheetRowsForGroup(gAfter, remaining)
				if err := s.sheets.AppendPaidAndRemovePending(ctx, groupID, rows); err != nil {
					log.Printf("billing: re-sync sheet for group %s after camper convert: %v", groupID, err)
				}
			}
		}
	}

	return summary, nil
}

// UpdateDayPassSummary captures the outcome of editing a day-pass camper's details.
type UpdateDayPassSummary struct {
	CamperName string
}

// UpdateDayPassCamper updates non-billing day-pass fields (shirt, catering, dietary).
func (s *Service) UpdateDayPassCamper(
	ctx context.Context,
	groupID, camperID, actor string,
	expectedVersion int,
	req UpdateDayPassCamperRequest,
) (UpdateDayPassSummary, error) {
	g, err := s.repo.FindGroupByID(ctx, groupID)
	if err != nil {
		return UpdateDayPassSummary{}, commonerrors.Internal(err.Error())
	}
	if g == nil {
		return UpdateDayPassSummary{}, commonerrors.NotFound("group not found")
	}

	campers, err := s.repo.CampersForGroup(ctx, groupID)
	if err != nil {
		return UpdateDayPassSummary{}, commonerrors.Internal(err.Error())
	}

	var target *domain.Camper
	for i := range campers {
		if campers[i].ID == camperID {
			target = &campers[i]
			break
		}
	}
	if target == nil {
		return UpdateDayPassSummary{}, commonerrors.NotFound("camper not found")
	}

	summary := UpdateDayPassSummary{
		CamperName: strings.TrimSpace(target.FirstName + " " + target.LastName),
	}

	if target.AttendanceType != domain.AttendanceDayPass {
		return UpdateDayPassSummary{}, commonerrors.BadRequest("only day-pass campers can be edited here", nil)
	}

	fields := map[string]string{}
	registration.ValidateDayPass("attendance", domain.AttendanceDTO{
		Type:          domain.AttendanceDayPass,
		Days:          target.DayPassDays,
		TshirtOption:  req.TshirtOption,
		ShirtSize:     req.ShirtSize,
		NeedsCatering: req.NeedsCatering,
	}, fields)
	if len(fields) > 0 {
		return UpdateDayPassSummary{}, commonerrors.ValidationFailed(fields)
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
		return UpdateDayPassSummary{}, mapVersionErr(ctx, s.repo, groupID, err)
	}

	if g.PaymentStatus == domain.PaymentPaid {
		remaining, err := s.repo.CampersForGroup(ctx, groupID)
		if err != nil {
			log.Printf("billing: load campers after day-pass edit %s/%s: %v", groupID, camperID, err)
		} else {
			gAfter, err := s.repo.FindGroupByID(ctx, groupID)
			if err != nil || gAfter == nil {
				log.Printf("billing: reload group after day-pass edit %s: %v", groupID, err)
			} else {
				if err := s.sheets.RemoveByGroupID(ctx, groupID); err != nil {
					log.Printf("billing: remove group %s from sheet after day-pass edit: %v", groupID, err)
				}
				rows := sheetRowsForGroup(gAfter, remaining)
				if err := s.sheets.AppendPaidAndRemovePending(ctx, groupID, rows); err != nil {
					log.Printf("billing: re-sync sheet for group %s after day-pass edit: %v", groupID, err)
				}
			}
		}
	}

	return summary, nil
}

// UpdateCamperCoachSummary captures the outcome of toggling one camper's coach seat.
type UpdateCamperCoachSummary struct {
	CamperName    string `json:"camper_name"`
	NeedsCoach    bool   `json:"needs_coach"`
	InvoiceVoided bool   `json:"invoice_voided,omitempty"`
	NoOp          bool   `json:"-"`
}

// UpdateCamperCoach sets whether one full-week camper travels on the coach.
func (s *Service) UpdateCamperCoach(
	ctx context.Context,
	groupID, camperID, actor string,
	expectedVersion int,
	req UpdateCamperCoachRequest,
) (UpdateCamperCoachSummary, error) {
	g, err := s.repo.FindGroupByID(ctx, groupID)
	if err != nil {
		return UpdateCamperCoachSummary{}, commonerrors.Internal(err.Error())
	}
	if g == nil {
		return UpdateCamperCoachSummary{}, commonerrors.NotFound("group not found")
	}

	campers, err := s.repo.CampersForGroup(ctx, groupID)
	if err != nil {
		return UpdateCamperCoachSummary{}, commonerrors.Internal(err.Error())
	}

	var target *domain.Camper
	for i := range campers {
		if campers[i].ID == camperID {
			target = &campers[i]
			break
		}
	}
	if target == nil {
		return UpdateCamperCoachSummary{}, commonerrors.NotFound("camper not found")
	}

	summary := UpdateCamperCoachSummary{
		CamperName: strings.TrimSpace(target.FirstName + " " + target.LastName),
		NeedsCoach: req.NeedsCoach,
	}

	if target.AttendanceType != domain.AttendanceFullWeek {
		return UpdateCamperCoachSummary{}, commonerrors.BadRequest("only full-week campers can use the coach", nil)
	}

	current := target.NeedsCoach != nil && *target.NeedsCoach
	if current == req.NeedsCoach {
		summary.NoOp = true
		return summary, nil
	}

	if g.CoachFeePaidAt != nil ||
		(g.CoachIncludedInBalance && g.BillingStatus == domain.BillingBalancePaid) {
		return UpdateCamperCoachSummary{}, commonerrors.BadRequest(
			"coach fee already paid; handle any refund in Stripe first", nil)
	}
	if g.CoachIncludedInBalance {
		return UpdateCamperCoachSummary{}, commonerrors.BadRequest(
			"coach fee is included in the balance invoice for this group", nil)
	}

	invoiceVoided := false
	if g.StripeCoachInvoiceID != nil && strings.TrimSpace(*g.StripeCoachInvoiceID) != "" && g.CoachFeePaidAt == nil {
		if err := s.stripe.VoidInvoiceIdempotent(ctx, *g.StripeCoachInvoiceID); err != nil {
			return UpdateCamperCoachSummary{}, commonerrors.BadRequest(
				fmt.Sprintf("coach invoice void failed; coach seat was not updated: %v", err), nil)
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
		return UpdateCamperCoachSummary{}, mapVersionErr(ctx, s.repo, groupID, err)
	}

	if g.PaymentStatus == domain.PaymentPaid {
		remaining, err := s.repo.CampersForGroup(ctx, groupID)
		if err != nil {
			log.Printf("billing: load campers after coach edit %s/%s: %v", groupID, camperID, err)
		} else {
			gAfter, err := s.repo.FindGroupByID(ctx, groupID)
			if err != nil || gAfter == nil {
				log.Printf("billing: reload group after coach edit %s: %v", groupID, err)
			} else {
				if err := s.sheets.RemoveByGroupID(ctx, groupID); err != nil {
					log.Printf("billing: remove group %s from sheet after coach edit: %v", groupID, err)
				}
				rows := sheetRowsForGroup(gAfter, remaining)
				if err := s.sheets.AppendPaidAndRemovePending(ctx, groupID, rows); err != nil {
					log.Printf("billing: re-sync sheet for group %s after coach edit: %v", groupID, err)
				}
			}
		}
	}

	return summary, nil
}

func sheetRowsForGroup(g *domain.Group, campers []domain.Camper) []sheets.Row {
	rows := make([]sheets.Row, 0, len(campers))
	for _, c := range campers {
		rows = append(rows, sheets.Row{
			GroupID:                   g.ID,
			PaymentStatus:             g.PaymentStatus,
			SubmittedAt:               g.CreatedAt,
			PaidAt:                    g.PaidAt,
			TotalAmountPence:          g.TotalAmountPence,
			Currency:                  g.Currency,
			ContactFirstName:          g.ContactFirstName,
			ContactLastName:           g.ContactLastName,
			ContactEmail:              g.ContactEmail,
			ContactPhone:              g.ContactPhone,
			IsMainContact:             c.IsMainContact,
			FirstName:                 c.FirstName,
			LastName:                  c.LastName,
			Gender:                    c.Gender,
			Age:                       c.Age,
			CellLeaderName:            c.CellLeaderName,
			IsCellLeader:              c.IsCellLeader,
			AttendanceType:            c.AttendanceType,
			ShirtSize:                 c.ShirtSize,
			DietaryRequirements:       c.DietaryRequirements,
			NeedsCoach:                c.NeedsCoach,
			AccommodationFirstChoice:  c.AccommodationFirstChoice,
			AccommodationSecondChoice: c.AccommodationSecondChoice,
			RoommateRequests:          c.RoommateRequests,
			DayPassDays:               c.DayPassDays,
			DayPassTshirtOption:       c.DayPassTshirtOption,
			DayPassNeedsCatering:      c.DayPassNeedsCatering,
		})
	}
	return rows
}

// ResyncSheet rewrites this group's rows in the Google Sheet from current DB
// state (remove then re-append). For paid groups this updates the Paid tab; for
// pending deposit groups, the Pending tab.
func (s *Service) ResyncSheet(ctx context.Context, groupID, actor string, expectedVersion int) error {
	g, err := s.repo.FindGroupByID(ctx, groupID)
	if err != nil {
		return commonerrors.Internal(err.Error())
	}
	if g == nil {
		return commonerrors.NotFound("group not found")
	}
	switch g.PaymentStatus {
	case domain.PaymentPaid, domain.PaymentPending:
	default:
		return commonerrors.BadRequest(
			"only pending or paid registrations can be re-synced to the sheet", nil)
	}

	campers, err := s.repo.CampersForGroup(ctx, groupID)
	if err != nil {
		return commonerrors.Internal(err.Error())
	}
	if len(campers) == 0 {
		return commonerrors.BadRequest("registration has no campers", nil)
	}

	rows := sheetRowsForGroup(g, campers)
	if err := s.sheets.RemoveByGroupID(ctx, groupID); err != nil {
		return commonerrors.Internal(err.Error())
	}
	switch g.PaymentStatus {
	case domain.PaymentPaid:
		err = s.sheets.AppendPaidAndRemovePending(ctx, groupID, rows)
	case domain.PaymentPending:
		err = s.sheets.AppendPending(ctx, rows)
	}
	if err != nil {
		return commonerrors.Internal(err.Error())
	}

	meta := domain.ActionMeta{
		Actor:           actor,
		Action:          "sheet_resynced",
		ExpectedVersion: expectedVersion,
	}
	if err := s.repo.StampOnly(ctx, groupID, meta); err != nil {
		return mapVersionErr(ctx, s.repo, groupID, err)
	}
	return nil
}

// ResyncAllSheets re-syncs every paid and pending registration to the Google
// Sheet from current DB state. Returns the success count and per-group errors.
func (s *Service) ResyncAllSheets(ctx context.Context, actor string) (int, map[string]string) {
	errs := map[string]string{}
	synced := 0
	for _, status := range []string{domain.PaymentPaid, domain.PaymentPending} {
		groups, err := s.repo.ListWithBilling(ctx, domain.ListFilterBilling{PaymentStatus: status})
		if err != nil {
			errs["_list"] = err.Error()
			return synced, errs
		}
		for _, g := range groups {
			if err := s.ResyncSheet(ctx, g.ID, actor, domain.SkipVersionCheck); err != nil {
				errs[g.ID] = err.Error()
				continue
			}
			synced++
		}
	}
	return synced, errs
}

// DeleteSummary captures Stripe cleanup performed when a registration is deleted.
type DeleteSummary struct {
	ContactName     string
	ContactEmail    string
	DepositRefunded bool
	BalanceRefunded bool
	InvoiceVoided   bool
	AmountPence     int64
}

// DeleteRegistration refunds/voids Stripe state, permanently removes the group
// row, and best-effort cleans up the Google Sheet. Aborts before delete if any
// non-idempotent Stripe step fails.
func (s *Service) DeleteRegistration(ctx context.Context, groupID, actor string, expectedVersion int) (DeleteSummary, error) {
	g, err := s.repo.FindGroupByID(ctx, groupID)
	if err != nil {
		return DeleteSummary{}, commonerrors.Internal(err.Error())
	}
	if g == nil {
		return DeleteSummary{}, commonerrors.NotFound("group not found")
	}

	summary := DeleteSummary{
		ContactName:  strings.TrimSpace(g.ContactFirstName + " " + g.ContactLastName),
		ContactEmail: g.ContactEmail,
	}

	var refundTotal int64

	if g.PaymentStatus == domain.PaymentPaid && g.StripePaymentIntentID != nil && *g.StripePaymentIntentID != "" {
		amt, err := s.stripe.RefundPaymentIntent(ctx, *g.StripePaymentIntentID)
		if err != nil {
			return DeleteSummary{}, commonerrors.BadRequest(
				fmt.Sprintf("deposit refund failed; registration was not deleted: %v", err), nil)
		}
		summary.DepositRefunded = true
		refundTotal += amt
	}

	if g.BillingStatus == domain.BillingBalancePaid && g.StripeInvoiceID != nil && *g.StripeInvoiceID != "" {
		amt, err := s.stripe.RefundInvoice(ctx, *g.StripeInvoiceID)
		if err != nil {
			return DeleteSummary{}, commonerrors.BadRequest(
				fmt.Sprintf("balance refund failed; registration was not deleted: %v", err), nil)
		}
		summary.BalanceRefunded = true
		refundTotal += amt
	}

	if g.BillingStatus == domain.BillingInvoiced && g.StripeInvoiceID != nil && *g.StripeInvoiceID != "" {
		if err := s.stripe.VoidInvoiceIdempotent(ctx, *g.StripeInvoiceID); err != nil {
			return DeleteSummary{}, commonerrors.BadRequest(
				fmt.Sprintf("invoice void failed; registration was not deleted: %v", err), nil)
		}
		summary.InvoiceVoided = true
	}

	// Void any open (unpaid) separate coach invoice so it can't be paid after
	// the registration is gone. Paid coach fees are refunded manually in Stripe.
	if g.StripeCoachInvoiceID != nil && *g.StripeCoachInvoiceID != "" && g.CoachFeePaidAt == nil {
		if err := s.stripe.VoidInvoiceIdempotent(ctx, *g.StripeCoachInvoiceID); err != nil {
			return DeleteSummary{}, commonerrors.BadRequest(
				fmt.Sprintf("coach invoice void failed; registration was not deleted: %v", err), nil)
		}
	}

	summary.AmountPence = refundTotal

	meta := domain.ActionMeta{
		Actor:           actor,
		Action:          "registration_deleted",
		ExpectedVersion: expectedVersion,
	}
	if err := s.repo.DeleteGroupMeta(ctx, groupID, meta); err != nil {
		return DeleteSummary{}, mapVersionErr(ctx, s.repo, groupID, err)
	}

	if err := s.sheets.RemoveByGroupID(ctx, groupID); err != nil {
		log.Printf("billing: remove group %s from sheet: %v", groupID, err)
	}

	return summary, nil
}

// ResendInvoice re-sends the Stripe invoice email.
func (s *Service) ResendInvoice(ctx context.Context, groupID, actor string, expectedVersion int) error {
	g, err := s.repo.FindGroupByID(ctx, groupID)
	if err != nil {
		return commonerrors.Internal(err.Error())
	}
	if g == nil {
		return commonerrors.NotFound("group not found")
	}
	if g.StripeInvoiceID == nil || *g.StripeInvoiceID == "" {
		return commonerrors.BadRequest("no invoice on file", nil)
	}
	if g.BillingStatus != domain.BillingInvoiced {
		return commonerrors.BadRequest("invoice is not open", nil)
	}
	// Primary: ask Stripe to re-email. If it succeeds, stamp and return.
	if err := s.stripe.SendInvoiceEmail(ctx, *g.StripeInvoiceID); err == nil {
		meta := domain.ActionMeta{
			Actor:           actor,
			Action:          "invoice_resent",
			ExpectedVersion: expectedVersion,
		}
		if err := s.repo.StampOnly(ctx, groupID, meta); err != nil {
			return mapVersionErr(ctx, s.repo, groupID, err)
		}
		return nil
	} else {
		log.Printf("billing: Stripe re-send failed for group %s; falling back to Resend: %v", groupID, err)
	}
	// Fallback: email the hosted payment link ourselves.
	res, err := s.stripe.GetInvoice(ctx, *g.StripeInvoiceID)
	if err != nil {
		return commonerrors.Internal(err.Error())
	}
	due := ""
	if g.InvoiceDueAt != nil {
		due = g.InvoiceDueAt.Format("2 Jan 2006")
	} else if !res.DueAt.IsZero() {
		due = res.DueAt.Format("2 Jan 2006")
	}
	campers, _ := s.repo.CampersForGroup(ctx, groupID)
	coachCount := int64(0)
	if g.CoachIncludedInBalance {
		coachCount = coachEligibleCount(campers)
	}
	if err := s.mailer.SendBalanceInvoice(ctx, email.BalanceInvoice{
		ToEmail:     g.ContactEmail,
		ToName:      g.ContactFirstName,
		PayURL:      res.HostedURL,
		DueDate:     due,
		AmountLabel: formatAmountLabel(res.AmountDuePence, res.Currency),
		Items:       balanceInvoiceItems(campers, s.accommodationNames(ctx), s.unitNames(ctx), coachCount),
	}); err != nil {
		return commonerrors.Internal(err.Error())
	}
	meta := domain.ActionMeta{
		Actor:           actor,
		Action:          "invoice_resent",
		ExpectedVersion: expectedVersion,
	}
	if err := s.repo.StampOnly(ctx, groupID, meta); err != nil {
		return mapVersionErr(ctx, s.repo, groupID, err)
	}
	return nil
}

// ExtendDueAt updates the stored due date (Stripe due date is informational after send).
func (s *Service) ExtendDueAt(ctx context.Context, groupID, actor string, expectedVersion int, dueAt time.Time) error {
	meta := domain.ActionMeta{
		Actor:           actor,
		Action:          "extend_due",
		ExpectedVersion: expectedVersion,
	}
	if err := s.repo.ExtendInvoiceDueAtMeta(ctx, groupID, dueAt, meta); err != nil {
		return mapVersionErr(ctx, s.repo, groupID, err)
	}
	return nil
}

// HandleInvoicePaid marks balance paid (idempotent) and notifies the White
// Team with a detailed summary of who paid, what it covered, and how much.
func (s *Service) HandleInvoicePaid(ctx context.Context, stripeInvoiceID, groupID string, amountPaidPence int64, currency string) error {
	g, err := s.repo.FindGroupByID(ctx, groupID)
	if err != nil {
		return err
	}
	if g == nil {
		g, err = s.repo.FindGroupByStripeInvoiceID(ctx, stripeInvoiceID)
		if err != nil || g == nil {
			return fmt.Errorf("group not found for invoice %s", stripeInvoiceID)
		}
	}
	if g.BillingStatus == domain.BillingBalancePaid {
		return nil
	}
	if err := s.repo.MarkBalancePaidMeta(ctx, g.ID, domain.ActionMeta{
		Actor:           "Stripe",
		Action:          "balance_paid",
		ExpectedVersion: domain.SkipVersionCheck,
	}); err != nil {
		return err
	}
	campers, _ := s.repo.CampersForGroup(ctx, g.ID)
	accNames := s.accommodationNames(ctx)
	coachCount := int64(0)
	if g.CoachIncludedInBalance {
		coachCount = coachEligibleCount(campers)
	}
	items := balanceInvoiceItems(campers, accNames, s.unitNames(ctx), coachCount)
	changes := offPreferenceChanges(campers, accNames)
	amountLabel := formatAmountLabel(amountPaidPence, currency)

	// Confirm to the family that their place is fully paid and confirmed.
	if err := s.mailer.SendBalancePaidConfirmation(ctx, email.BalancePaidConfirmation{
		ToEmail:     g.ContactEmail,
		ToName:      g.ContactFirstName,
		AmountLabel: amountLabel,
		Items:       items,
		Changes:     changes,
	}); err != nil {
		log.Printf("billing: balance-paid confirmation email to %s: %v", g.ContactEmail, err)
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
	return nil
}

// HandleInvoiceFailed logs a failed balance payment attempt.
func (s *Service) HandleInvoiceFailed(ctx context.Context, groupID string) {
	log.Printf("billing: invoice payment failed for group %s", groupID)
}

// HandleInvoiceUncollectible flags an invoice as uncollectible.
func (s *Service) HandleInvoiceUncollectible(ctx context.Context, groupID string) {
	log.Printf("billing: invoice marked uncollectible for group %s", groupID)
}

// SweepOverdue releases all groups past invoice_due_at still in invoiced status.
func (s *Service) SweepOverdue(ctx context.Context) (int, error) {
	overdue, err := s.repo.ListOverdueInvoiced(ctx, time.Now().UTC())
	if err != nil {
		return 0, err
	}
	n := 0
	for _, g := range overdue {
		if err := s.VoidAndRelease(ctx, g.ID, "unpaid after invoice due date", "system", domain.SkipVersionCheck, false); err != nil {
			log.Printf("billing: sweep release group %s: %v", g.ID, err)
			continue
		}
		n++
	}
	return n, nil
}

func camperNames(campers []domain.Camper) []string {
	var names []string
	for _, c := range campers {
		names = append(names, c.FirstName+" "+c.LastName)
	}
	return names
}

// accommodationNames returns a code->display_name lookup for invoice line items.
func (s *Service) accommodationNames(ctx context.Context) map[string]string {
	m := map[string]string{}
	types, err := s.repo.ListAccommodationTypes(ctx)
	if err != nil {
		log.Printf("billing: load accommodation names: %v", err)
		return m
	}
	for _, t := range types {
		m[t.Code] = t.DisplayName
	}
	return m
}

// unitNames returns a unit code->display_name lookup for invoice line items.
func (s *Service) unitNames(ctx context.Context) map[string]string {
	m := map[string]string{}
	units, err := s.repo.ListAccommodationUnits(ctx)
	if err != nil {
		log.Printf("billing: load unit names: %v", err)
		return m
	}
	for _, u := range units {
		m[u.Code] = u.DisplayName
	}
	return m
}

// balanceInvoiceItems produces one "Name — Accommodation [Unit]" line per
// full-week camper, for the invoice email body.
// dayPassLine returns the invoice line for a day-pass camper: the day-pass
// price charged once per selected day. ok is false for non-day-pass campers or
// when no days are recorded (validation guarantees >=1; defensive).
func dayPassLine(c domain.Camper, priceID string) (InvoiceLine, bool) {
	if c.AttendanceType != domain.AttendanceDayPass {
		return InvoiceLine{}, false
	}
	if len(c.DayPassDays) == 0 {
		return InvoiceLine{}, false
	}
	return InvoiceLine{PriceID: priceID, Quantity: int64(len(c.DayPassDays))}, true
}

// coachEligibleCount counts full-week campers who opted for the coach and are
// old enough to be charged. Children under MinDepositAge (4) ride free.
func coachEligibleCount(campers []domain.Camper) int64 {
	var n int64
	for _, c := range campers {
		if c.AttendanceType != domain.AttendanceFullWeek {
			continue
		}
		if c.NeedsCoach != nil && *c.NeedsCoach && c.Age >= registration.MinDepositAge {
			n++
		}
	}
	return n
}

// coachLine returns the single coach invoice line billing `count` passengers at
// the coach price. ok is false when there are no eligible passengers.
func coachLine(count int64, priceID string) (InvoiceLine, bool) {
	if count <= 0 {
		return InvoiceLine{}, false
	}
	return InvoiceLine{PriceID: priceID, Quantity: count}, true
}

func balanceInvoiceItems(campers []domain.Camper, accNames, unitNames map[string]string, coachCount int64) []string {
	var items []string
	for _, c := range campers {
		if c.AttendanceType == domain.AttendanceDayPass {
			if n := len(c.DayPassDays); n > 0 {
				unit := "day"
				if n != 1 {
					unit = "days"
				}
				items = append(items, fmt.Sprintf("%s %s — Day pass (%d %s)",
					c.FirstName, c.LastName, n, unit))
			}
			continue
		}
		if c.AttendanceType != domain.AttendanceFullWeek {
			continue
		}
		name := c.FirstName + " " + c.LastName
		if c.AllocatedAccommodationCode != nil {
			code := strings.TrimSpace(*c.AllocatedAccommodationCode)
			if code != "" {
				display := accNames[code]
				if display == "" {
					display = code
				}
				line := name + " — " + display
				if c.AllocatedUnitCode != nil {
					unitCode := strings.TrimSpace(*c.AllocatedUnitCode)
					if unitCode != "" {
						unitDisplay := unitNames[unitCode]
						if unitDisplay == "" {
							unitDisplay = unitCode
						}
						line += " (" + unitDisplay + ")"
					}
				}
				items = append(items, line)
				continue
			}
		}
		items = append(items, name)
	}
	if coachCount > 0 {
		items = append(items, fmt.Sprintf("Coach transport (×%d)", coachCount))
	}
	return items
}

func formatAmountLabel(pence int64, currency string) string {
	if pence <= 0 {
		return ""
	}
	symbol := "£"
	if cur := strings.ToUpper(currency); cur != "GBP" && cur != "" {
		symbol = cur + " "
	}
	return fmt.Sprintf("%s%d.%02d", symbol, pence/100, pence%100)
}

func strVal(p *string) string {
	if p == nil {
		return ""
	}
	return strings.TrimSpace(*p)
}

func accDisplayName(accNames map[string]string, code string) string {
	if code == "" {
		return ""
	}
	if display := accNames[code]; display != "" {
		return display
	}
	return code
}

// offPreferenceChanges returns campers whose allocated tier is neither their
// 1st nor 2nd choice. Skips when no preference is on record.
func offPreferenceChanges(campers []domain.Camper, accNames map[string]string) []email.AccommodationChange {
	var out []email.AccommodationChange
	for _, c := range campers {
		if c.AttendanceType != domain.AttendanceFullWeek {
			continue
		}
		alloc := strVal(c.AllocatedAccommodationCode)
		if alloc == "" {
			continue
		}
		first := strVal(c.AccommodationFirstChoice)
		second := strVal(c.AccommodationSecondChoice)
		if first == "" && second == "" {
			continue
		}
		if alloc == first || alloc == second {
			continue
		}
		out = append(out, email.AccommodationChange{
			CamperName:   c.FirstName + " " + c.LastName,
			FirstChoice:  accDisplayName(accNames, first),
			SecondChoice: accDisplayName(accNames, second),
			Allocated:    accDisplayName(accNames, alloc),
			TentGuidance: alloc == registration.AccommodationTent,
		})
	}
	return out
}

// sendAccommodationChangedNotice emails the family when invoiced for
// off-preference accommodation. Best-effort: must not fail the invoice send.
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
