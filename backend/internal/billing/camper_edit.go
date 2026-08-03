package billing

import (
	"context"
	"fmt"
	"log"
	"strings"

	"pagasacentre/backend/internal/email"
	"pagasacentre/backend/internal/registration"
	"pagasacentre/backend/internal/registration/domain"
	"pagasacentre/backend/internal/registration/storage"
	commonerrors "pagasacentre/backend/pkg/commonlibrary/errors"
)

// EditCamperRequest replaces one camper's details wholesale. Every editable
// field is sent on every request, so a rule can never be skipped by leaving a
// field out — the alternative, merging whatever turns up, means validation only
// sees half the person and can pass a combination that is actually invalid.
//
// The full-week fields are ignored for day visitors, whose stay details belong
// to the day-pass editor.
type EditCamperRequest struct {
	ExpectedVersion *int `json:"expected_version,omitempty"`

	FirstName      string `json:"first_name"`
	LastName       string `json:"last_name"`
	Gender         string `json:"gender"`
	Age            int    `json:"age"`
	CellLeaderName string `json:"cell_leader_name"`
	IsCellLeader   bool   `json:"is_cell_leader"`

	ShirtSize                  string `json:"shirt_size"`
	DietaryRequirements        string `json:"dietary_requirements"`
	AccommodationFirstChoice   string `json:"accommodation_first_choice"`
	AccommodationSecondChoice  string `json:"accommodation_second_choice"`
	AllocatedAccommodationCode string `json:"allocated_accommodation_code"`
	AllocatedUnitCode          string `json:"allocated_unit_code"`
}

// AddCamperRequest is POST .../campers.
type AddCamperRequest struct {
	ExpectedVersion *int             `json:"expected_version,omitempty"`
	Camper          domain.CamperDTO `json:"camper"`
}

// EditCamperSummary tells the dashboard what the edit actually did, so it can
// say whether an invoice was cancelled rather than leaving the admin to find out
// from the family.
type EditCamperSummary struct {
	CamperName    string `json:"camper_name"`
	PreviousName  string `json:"previous_name"`
	InvoiceVoided bool   `json:"invoice_voided"`
	Repriced      bool   `json:"repriced"`
}

// AddCamperSummary reports the new camper and what their arrival cost the group.
type AddCamperSummary struct {
	CamperID         string `json:"camper_id"`
	CamperName       string `json:"camper_name"`
	DepositOwedPence int    `json:"deposit_owed_pence"`
	InvoiceVoided    bool   `json:"invoice_voided"`
	NeedsAllocation  bool   `json:"needs_allocation"`
}

// EditCamper rewrites one camper's details in place. This is how a swap is done:
// the family sends someone else, and the person in that seat becomes the new
// person. Editing rather than remove-then-add keeps the seat, its deposit and
// its allocation intact, which is the whole reason a swap should not cost the
// family anything.
//
// An open invoice is only cancelled when the edit changes what is owed. Fixing a
// misspelled name leaves a live payment link alone.
func (s *Service) EditCamper(
	ctx context.Context,
	groupID, camperID, actor string,
	expectedVersion int,
	req EditCamperRequest,
) (EditCamperSummary, error) {
	g, err := s.repo.FindGroupByID(ctx, groupID)
	if err != nil {
		return EditCamperSummary{}, commonerrors.Internal(err.Error())
	}
	if g == nil {
		return EditCamperSummary{}, commonerrors.NotFound("group not found")
	}
	campers, err := s.repo.CampersForGroup(ctx, groupID)
	if err != nil {
		return EditCamperSummary{}, commonerrors.Internal(err.Error())
	}
	target := findCamper(campers, camperID)
	if target == nil {
		return EditCamperSummary{}, commonerrors.NotFound("camper not found")
	}

	fullWeek := target.AttendanceType == domain.AttendanceFullWeek
	if err := validateEdit(*target, req, fullWeek); err != nil {
		return EditCamperSummary{}, err
	}

	details := storage.CamperDetails{
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
			if err := s.validateAllocatedUnit(ctx, allocCode, unitCode); err != nil {
				return EditCamperSummary{}, err
			}
		} else if unitCode != "" {
			return EditCamperSummary{}, commonerrors.BadRequest(
				"cannot set a unit without an accommodation", nil)
		}

		priceBefore, err := s.priceFor(ctx, strVal(target.AllocatedAccommodationCode), target.Age, g.IsFree)
		if err != nil {
			return EditCamperSummary{}, err
		}
		priceAfter, err := s.priceFor(ctx, allocCode, req.Age, g.IsFree)
		if err != nil {
			return EditCamperSummary{}, err
		}
		repriced = priceBefore != priceAfter ||
			coachEligible(*target) != coachEligibleAtAge(*target, req.Age)

		details.ShirtSize = optStr(req.ShirtSize)
		details.DietaryRequirements = optStr(req.DietaryRequirements)
		details.AccommodationFirstChoice = optStr(req.AccommodationFirstChoice)
		details.AccommodationSecondChoice = optStr(req.AccommodationSecondChoice)
		details.AllocatedAccommodationCode = optStr(allocCode)
		details.AllocatedUnitCode = optStr(unitCode)
		details.BilledStripePriceID = optStr(priceAfter)
	}

	if repriced && g.BillingStatus == domain.BillingBalancePaid {
		return EditCamperSummary{}, commonerrors.BadRequest(
			"this balance is already paid, so changes that alter what they owe "+
				"have to be settled in Stripe; details that don't change the price "+
				"can still be edited", nil)
	}

	summary := EditCamperSummary{
		CamperName:   strings.TrimSpace(details.FirstName + " " + details.LastName),
		PreviousName: strings.TrimSpace(target.FirstName + " " + target.LastName),
		Repriced:     repriced,
	}

	newStatus := ""
	if repriced && g.BillingStatus == domain.BillingInvoiced &&
		g.StripeInvoiceID != nil && *g.StripeInvoiceID != "" {
		if err := s.stripe.VoidInvoiceIdempotent(ctx, *g.StripeInvoiceID); err != nil {
			return EditCamperSummary{}, commonerrors.BadRequest(
				fmt.Sprintf("invoice void failed; nothing was changed: %v", err), nil)
		}
		summary.InvoiceVoided = true
		newStatus = allocatedOrNone(campersAfterEdit(campers, camperID, details))
	}

	meta := domain.ActionMeta{
		Actor:           actor,
		Action:          "camper_edited",
		ExpectedVersion: expectedVersion,
	}
	if err := s.repo.UpdateCamperDetailsMeta(ctx, groupID, camperID, details, newStatus, meta); err != nil {
		return EditCamperSummary{}, mapVersionErr(ctx, s.repo, groupID, err)
	}

	s.resyncSheet(ctx, g, "camper edit")
	s.sendBookingUpdated(ctx, groupID, g, "", summary.InvoiceVoided)
	return summary, nil
}

// AddCamper puts another person on an existing booking. They owe the deposit
// everyone else already paid, recorded against them and billed on the balance
// invoice, so nobody gets a cheaper place for arriving late.
func (s *Service) AddCamper(
	ctx context.Context,
	groupID, actor string,
	expectedVersion int,
	req AddCamperRequest,
) (AddCamperSummary, error) {
	g, err := s.repo.FindGroupByID(ctx, groupID)
	if err != nil {
		return AddCamperSummary{}, commonerrors.Internal(err.Error())
	}
	if g == nil {
		return AddCamperSummary{}, commonerrors.NotFound("group not found")
	}

	if g.PaymentStatus != domain.PaymentPaid {
		return AddCamperSummary{}, commonerrors.BadRequest(
			"this group has not paid their deposit yet; wait for it to clear before adding anyone", nil)
	}
	if g.IsFree {
		return AddCamperSummary{}, commonerrors.BadRequest(
			"church-sponsored registrations are agreed as a whole; add this person as their own registration", nil)
	}
	if g.PaidInFullAtRegistration {
		return AddCamperSummary{}, commonerrors.BadRequest(
			"this family paid for camp in full at registration; add this person as their own registration", nil)
	}
	if g.BillingStatus == domain.BillingBalancePaid {
		return AddCamperSummary{}, commonerrors.BadRequest(
			"this balance is already paid, so there is no invoice left to charge them on; "+
				"add this person as their own registration", nil)
	}

	c := req.Camper
	c.IsMainContact = false
	fields := map[string]string{}
	registration.ValidateCamper("camper", c, fields)
	if len(fields) > 0 {
		return AddCamperSummary{}, commonerrors.ValidationFailed(fields)
	}

	depositOwed := 0
	if c.Attendance.Type == domain.AttendanceFullWeek && c.Age >= registration.MinDepositAge {
		depositOwed = s.depositAmountPence(ctx)
	}

	summary := AddCamperSummary{
		CamperName:       strings.TrimSpace(c.FirstName + " " + c.LastName),
		DepositOwedPence: depositOwed,
	}

	campers, err := s.repo.CampersForGroup(ctx, groupID)
	if err != nil {
		return AddCamperSummary{}, commonerrors.Internal(err.Error())
	}

	// A new full-week camper has no bed. Sending the group back to needing
	// accommodation is what puts them in front of the White Team again, rather
	// than leaving someone quietly unplaced on an allocated group.
	addedFullWeek := c.Attendance.Type == domain.AttendanceFullWeek
	newStatus := ""
	if g.BillingStatus == domain.BillingInvoiced &&
		g.StripeInvoiceID != nil && *g.StripeInvoiceID != "" {
		if err := s.stripe.VoidInvoiceIdempotent(ctx, *g.StripeInvoiceID); err != nil {
			return AddCamperSummary{}, commonerrors.BadRequest(
				fmt.Sprintf("invoice void failed; nobody was added: %v", err), nil)
		}
		summary.InvoiceVoided = true
		newStatus = domain.BillingNone
		if !addedFullWeek {
			newStatus = allocatedOrNone(campers)
		}
	} else if addedFullWeek && g.BillingStatus == domain.BillingAllocated {
		newStatus = domain.BillingNone
	}
	summary.NeedsAllocation = addedFullWeek

	meta := domain.ActionMeta{
		Actor:           actor,
		Action:          "camper_added",
		ExpectedVersion: expectedVersion,
	}
	camperID, err := s.repo.AddCamperMeta(ctx, groupID, c, depositOwed, newStatus, meta)
	if err != nil {
		return AddCamperSummary{}, mapVersionErr(ctx, s.repo, groupID, err)
	}
	summary.CamperID = camperID

	s.resyncSheet(ctx, g, "camper add")
	s.sendBookingUpdated(ctx, groupID, g, summary.CamperName, summary.InvoiceVoided)
	return summary, nil
}

// MakeMainContact moves the main-contact marker to another camper. The marker
// only says which camper the booking is filed under; the contact name, email and
// phone the White Team writes to are the group's own and are untouched.
func (s *Service) MakeMainContact(
	ctx context.Context,
	groupID, camperID, actor string,
	expectedVersion int,
) (string, error) {
	g, err := s.repo.FindGroupByID(ctx, groupID)
	if err != nil {
		return "", commonerrors.Internal(err.Error())
	}
	if g == nil {
		return "", commonerrors.NotFound("group not found")
	}
	campers, err := s.repo.CampersForGroup(ctx, groupID)
	if err != nil {
		return "", commonerrors.Internal(err.Error())
	}
	target := findCamper(campers, camperID)
	if target == nil {
		return "", commonerrors.NotFound("camper not found")
	}
	name := strings.TrimSpace(target.FirstName + " " + target.LastName)
	if target.IsMainContact {
		return name, nil
	}

	meta := domain.ActionMeta{
		Actor:           actor,
		Action:          "main_contact_moved",
		ExpectedVersion: expectedVersion,
	}
	if err := s.repo.SetMainContactMeta(ctx, groupID, camperID, meta); err != nil {
		return "", mapVersionErr(ctx, s.repo, groupID, err)
	}
	s.resyncSheet(ctx, g, "main contact change")
	return name, nil
}

// WaiveCamperDeposit clears a deposit owed by a late arrival so it is never
// billed. This is the remedy when a swap was done the long way round — someone
// removed and someone else added — and the family should not pay twice for one
// seat.
func (s *Service) WaiveCamperDeposit(
	ctx context.Context,
	groupID, camperID, actor string,
	expectedVersion int,
) (string, error) {
	g, err := s.repo.FindGroupByID(ctx, groupID)
	if err != nil {
		return "", commonerrors.Internal(err.Error())
	}
	if g == nil {
		return "", commonerrors.NotFound("group not found")
	}
	if g.BillingStatus == domain.BillingBalancePaid {
		return "", commonerrors.BadRequest(
			"this balance is already paid; there is no invoice left to take it off", nil)
	}
	campers, err := s.repo.CampersForGroup(ctx, groupID)
	if err != nil {
		return "", commonerrors.Internal(err.Error())
	}
	target := findCamper(campers, camperID)
	if target == nil {
		return "", commonerrors.NotFound("camper not found")
	}
	name := strings.TrimSpace(target.FirstName + " " + target.LastName)
	if target.DepositOwedPence <= 0 {
		return name, nil
	}

	meta := domain.ActionMeta{
		Actor:           actor,
		Action:          "camper_deposit_waived",
		ExpectedVersion: expectedVersion,
	}
	if err := s.repo.WaiveCamperDepositMeta(ctx, groupID, camperID, meta); err != nil {
		return "", mapVersionErr(ctx, s.repo, groupID, err)
	}
	return name, nil
}

// validateEdit runs the full camper ruleset against the replacement details,
// plus the allocated-accommodation age limit that registration never needs
// because it only ever sees preferences.
func validateEdit(target domain.Camper, req EditCamperRequest, fullWeek bool) error {
	c := domain.CamperDTO{
		FirstName:      req.FirstName,
		LastName:       req.LastName,
		Gender:         req.Gender,
		Age:            req.Age,
		CellLeaderName: req.CellLeaderName,
		IsCellLeader:   req.IsCellLeader,
	}
	if fullWeek {
		c.Attendance = domain.AttendanceDTO{
			Type:                      domain.AttendanceFullWeek,
			ShirtSize:                 req.ShirtSize,
			DietaryRequirements:       req.DietaryRequirements,
			AccommodationFirstChoice:  req.AccommodationFirstChoice,
			AccommodationSecondChoice: req.AccommodationSecondChoice,
			RoommateRequests:          strVal(target.RoommateRequests),
			NeedsCoach:                target.NeedsCoach,
		}
	} else {
		// The day-pass fields are not editable here, so they are carried over
		// from the stored row to be validated as a whole with the new age.
		c.Attendance = domain.AttendanceDTO{
			Type:                domain.AttendanceDayPass,
			Days:                target.DayPassDays,
			TshirtOption:        strVal(target.DayPassTshirtOption),
			ShirtSize:           strVal(target.ShirtSize),
			DietaryRequirements: strVal(target.DietaryRequirements),
			NeedsCatering:       target.DayPassNeedsCatering,
		}
	}

	fields := map[string]string{}
	registration.ValidateCamper("camper", c, fields)
	if fullWeek {
		registration.ValidateAllocatedAccommodation(
			"camper.allocated_accommodation_code",
			req.AllocatedAccommodationCode, req.Age, fields)
	}
	if len(fields) > 0 {
		return commonerrors.ValidationFailed(fields)
	}
	return nil
}

// priceFor is the Stripe Price a camper's placement bills at, or empty when they
// have no placement or the group is sponsored. Used to decide whether an edit
// changed the money.
func (s *Service) priceFor(ctx context.Context, accommodationCode string, age int, isFree bool) (string, error) {
	code := strings.TrimSpace(accommodationCode)
	if code == "" || isFree {
		return "", nil
	}
	priceID, err := s.resolvePriceID(ctx, code, age)
	if err != nil {
		return "", commonerrors.BadRequest(err.Error(), nil)
	}
	return priceID, nil
}

func coachEligible(c domain.Camper) bool {
	return coachEligibleAtAge(c, c.Age)
}

// coachEligibleAtAge asks whether this camper would be billed a coach seat at
// the given age. Coach is only charged from MinDepositAge up, so ageing a child
// across that line changes what the group owes.
func coachEligibleAtAge(c domain.Camper, age int) bool {
	return c.AttendanceType == domain.AttendanceFullWeek &&
		c.NeedsCoach != nil && *c.NeedsCoach &&
		age >= registration.MinDepositAge
}

// allocatedOrNone is the billing status a group falls back to once its invoice
// is voided. It only claims to be allocated when every full-week camper actually
// has a bed; otherwise the group goes back to needing accommodation. Claiming
// "allocated" with someone unplaced strands the group — invoicing refuses it, and
// the fix is buried in a screen that says everything is done.
func allocatedOrNone(campers []domain.Camper) string {
	anyFullWeek := false
	for _, c := range campers {
		if c.AttendanceType != domain.AttendanceFullWeek {
			continue
		}
		anyFullWeek = true
		if strVal(c.AllocatedAccommodationCode) == "" {
			return domain.BillingNone
		}
	}
	if anyFullWeek {
		return domain.BillingAllocated
	}
	return domain.BillingNone
}

// campersAfterEdit is the roster as it will be once this edit lands, so the
// billing status is decided from the outcome rather than from what was there
// before the change.
func campersAfterEdit(
	campers []domain.Camper,
	camperID string,
	details storage.CamperDetails,
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

func findCamper(campers []domain.Camper, camperID string) *domain.Camper {
	for i := range campers {
		if campers[i].ID == camperID {
			return &campers[i]
		}
	}
	return nil
}

func optStr(s string) *string {
	t := strings.TrimSpace(s)
	if t == "" {
		return nil
	}
	return &t
}

// resyncSheet rewrites this group's rows in the Google Sheet. Only paid groups
// are on the sheet at all, so unpaid ones are skipped. Best-effort: the change
// is already committed and a sheet outage must not fail the action.
func (s *Service) resyncSheet(ctx context.Context, g *domain.Group, what string) {
	if g.PaymentStatus != domain.PaymentPaid {
		return
	}
	campers, err := s.repo.CampersForGroup(ctx, g.ID)
	if err != nil {
		log.Printf("billing: load campers for sheet after %s (group %s): %v", what, g.ID, err)
		return
	}
	gAfter, err := s.repo.FindGroupByID(ctx, g.ID)
	if err != nil || gAfter == nil {
		log.Printf("billing: reload group for sheet after %s (group %s): %v", what, g.ID, err)
		return
	}
	if err := s.sheets.RemoveByGroupID(ctx, g.ID); err != nil {
		log.Printf("billing: clear sheet rows after %s (group %s): %v", what, g.ID, err)
	}
	if err := s.sheets.AppendPaidAndRemovePending(ctx, g.ID, sheetRowsForGroup(gAfter, campers)); err != nil {
		log.Printf("billing: re-sync sheet after %s (group %s): %v", what, g.ID, err)
	}
}

// sendBookingUpdated tells the family who is on their booking now. Sent for both
// adds and edits: a swap looks like an edit from here, and a family who is not
// told will only find out when the wrong person is turned away at the gate.
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
	campers, err := s.repo.CampersForGroup(ctx, groupID)
	if err != nil {
		log.Printf("billing: load campers for booking-updated email (group %s): %v", groupID, err)
		return
	}
	if err := s.mailer.SendBookingUpdated(ctx, email.BookingUpdated{
		ToEmail:          g.ContactEmail,
		ToName:           g.ContactFirstName,
		Campers:          rosterItems(campers, s.accommodationNames(ctx), s.unitNames(ctx)),
		AddedName:        addedName,
		InvoiceCancelled: invoiceVoided,
	}); err != nil {
		log.Printf("billing: booking-updated email to %s: %v", g.ContactEmail, err)
	}
}
