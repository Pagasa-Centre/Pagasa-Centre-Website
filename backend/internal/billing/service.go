package billing

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"pagasacentre/backend/internal/email"
	"pagasacentre/backend/internal/registration"
	commonerrors "pagasacentre/backend/pkg/commonlibrary/errors"
	"pagasacentre/backend/internal/registration/domain"
	"pagasacentre/backend/internal/registration/storage"
)

// Service coordinates allocation, Stripe Invoices, and release sweeps.
type Service struct {
	repo   *storage.Repository
	stripe *StripeBilling
	mailer email.Mailer
	cfg    Config
}

func NewService(repo *storage.Repository, stripe *StripeBilling, mailer email.Mailer, cfg Config) *Service {
	if cfg.InvoiceDueDays <= 0 {
		cfg.InvoiceDueDays = 15
	}
	return &Service{repo: repo, stripe: stripe, mailer: mailer, cfg: cfg}
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
	if g.BillingStatus == domain.BillingBalancePaid {
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
		return commonerrors.BadRequest("balance already paid", nil)
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
	if accommodationCode == registration.AccommodationChild && age < registration.MinDepositAge {
		if s.cfg.StripePriceChildUnder3 == "" {
			return "", fmt.Errorf("STRIPE_PRICE_CHILD_UNDER3 is not configured")
		}
		return s.cfg.StripePriceChildUnder3, nil
	}
	t, err := s.repo.GetAccommodationType(ctx, accommodationCode)
	if err != nil {
		return "", err
	}
	if t == nil {
		return "", fmt.Errorf("unknown accommodation %q", accommodationCode)
	}
	if t.StripePriceID == nil || strings.TrimSpace(*t.StripePriceID) == "" {
		return "", fmt.Errorf("accommodation %q has no stripe_price_id configured", accommodationCode)
	}
	return strings.TrimSpace(*t.StripePriceID), nil
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
	if g.PaymentStatus != domain.PaymentPaid {
		return commonerrors.BadRequest("deposit must be paid first", nil)
	}
	switch g.BillingStatus {
	case domain.BillingBalancePaid:
		return commonerrors.BadRequest("balance already paid", nil)
	case domain.BillingInvoiced:
		return commonerrors.BadRequest("invoice already sent", nil)
	case domain.BillingNone, domain.BillingReleased:
		return commonerrors.BadRequest("group must be allocated before invoicing", nil)
	}

	campers, err := s.repo.CampersForGroup(ctx, groupID)
	if err != nil {
		return commonerrors.Internal(err.Error())
	}
	var priceIDs []string
	for _, c := range campers {
		if c.AttendanceType != domain.AttendanceFullWeek {
			continue
		}
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
		priceIDs = append(priceIDs, priceID)
	}
	if len(priceIDs) == 0 {
		return commonerrors.BadRequest("no full-week campers to invoice", nil)
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
		ctx, customerID, g.ID, priceIDs, int64(s.cfg.InvoiceDueDays))
	if err != nil {
		return commonerrors.Internal(err.Error())
	}
	meta := domain.ActionMeta{
		Actor:           actor,
		Action:          "invoice_sent",
		ExpectedVersion: expectedVersion,
	}
	if err := s.repo.SetInvoiceDetailsMeta(ctx, groupID, res.ID, res.DueAt, meta); err != nil {
		return mapVersionErr(ctx, s.repo, groupID, err)
	}
	// Stripe is the primary sender. Only if Stripe couldn't email the invoice
	// (e.g. restricted account) do we fall back to emailing the link ourselves.
	if !res.StripeEmailed {
		if err := s.mailer.SendBalanceInvoice(ctx, email.BalanceInvoice{
			ToEmail:     g.ContactEmail,
			ToName:      g.ContactFirstName,
			PayURL:      res.HostedURL,
			DueDate:     res.DueAt.Format("2 Jan 2006"),
			AmountLabel: formatAmountLabel(res.AmountDuePence, res.Currency),
			Items:       balanceInvoiceItems(campers, s.accommodationNames(ctx), s.unitNames(ctx)),
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
		if err := s.SendInvoice(ctx, id, actor, domain.SkipVersionCheck); err != nil {
			errs[id] = err.Error()
		}
	}
	return errs
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
	if err := s.mailer.SendSponsorshipConfirmed(ctx, email.SponsorshipConfirmed{
		ToEmail: g.ContactEmail,
		ToName:  g.ContactFirstName,
		Items:   balanceInvoiceItems(campers, s.accommodationNames(ctx), s.unitNames(ctx)),
	}); err != nil {
		log.Printf("billing: sponsorship confirmed email to %s: %v", g.ContactEmail, err)
	}
}

// VoidAndRelease voids the Stripe invoice (if any) and clears allocations.
func (s *Service) VoidAndRelease(ctx context.Context, groupID, reason, actor string, expectedVersion int) error {
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
	meta := domain.ActionMeta{
		Actor:           actor,
		Action:          "released",
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
	if err := s.mailer.SendBalanceInvoice(ctx, email.BalanceInvoice{
		ToEmail:     g.ContactEmail,
		ToName:      g.ContactFirstName,
		PayURL:      res.HostedURL,
		DueDate:     due,
		AmountLabel: formatAmountLabel(res.AmountDuePence, res.Currency),
		Items:       balanceInvoiceItems(campers, s.accommodationNames(ctx), s.unitNames(ctx)),
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
	items := balanceInvoiceItems(campers, s.accommodationNames(ctx), s.unitNames(ctx))
	amountLabel := formatAmountLabel(amountPaidPence, currency)

	// Confirm to the family that their place is fully paid and confirmed.
	if err := s.mailer.SendBalancePaidConfirmation(ctx, email.BalancePaidConfirmation{
		ToEmail:     g.ContactEmail,
		ToName:      g.ContactFirstName,
		AmountLabel: amountLabel,
		Items:       items,
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
		if err := s.VoidAndRelease(ctx, g.ID, "unpaid after invoice due date", "system", domain.SkipVersionCheck); err != nil {
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
func balanceInvoiceItems(campers []domain.Camper, accNames, unitNames map[string]string) []string {
	var items []string
	for _, c := range campers {
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
