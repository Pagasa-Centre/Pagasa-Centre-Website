package registration

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"pagasacentre/backend/internal/email"
	"pagasacentre/backend/internal/registration/domain"
	"pagasacentre/backend/internal/sheets"
)

type PriceLookup interface {
	GetPrice(ctx context.Context, code string) (domain.PriceRow, error)
}

type CheckoutCreator interface {
	CreateCheckoutSession(ctx context.Context, p CheckoutParams) (CheckoutSession, error)
}

type CheckoutParams struct {
	GroupID  string
	Email    string
	Currency string
	Lines    []CheckoutLine
}

type CheckoutSession struct {
	ID  string
	URL string
}

type CampConfigReader interface {
	RegistrationsOpen(ctx context.Context) (bool, error)
	RegistrationPaymentMode(ctx context.Context) (string, error)
}

type Service struct {
	repo          groupRepo
	tx            txRunner
	prices        PriceLookup
	checkout      CheckoutCreator
	priceAmounts  PriceAmounts
	camp          CampConfigReader
	mailer        email.Mailer
	sheets        sheets.Sync
	publicBaseURL string
	cfg           Config
}

func NewService(
	repo groupRepo,
	tx txRunner,
	prices PriceLookup,
	checkout CheckoutCreator,
	priceAmounts PriceAmounts,
	campCfg CampConfigReader,
	mailer email.Mailer,
	sheetSync sheets.Sync,
	publicBaseURL string,
	cfg Config,
) *Service {
	if sheetSync == nil {
		sheetSync = sheets.NewNoopSync()
	}
	return &Service{
		repo:          repo,
		tx:            tx,
		prices:        prices,
		checkout:      checkout,
		priceAmounts:  priceAmounts,
		camp:          campCfg,
		mailer:        mailer,
		sheets:        sheetSync,
		publicBaseURL: publicBaseURL,
		cfg:           cfg,
	}
}

func (s *Service) Submit(ctx context.Context, req domain.Submission) (*domain.SubmitResult, error) {
	if err := Validate(req); err != nil {
		return nil, err
	}

	open, err := s.camp.RegistrationsOpen(ctx)
	if err != nil {
		return nil, fmt.Errorf("operation: %w", err)
	}
	if !open {
		return nil, ErrRegistrationsClosed
	}

	if err := s.ensureAccommodationAvailable(ctx, req); err != nil {
		return nil, err
	}

	mode, err := s.camp.RegistrationPaymentMode(ctx)
	if err != nil {
		return nil, fmt.Errorf("operation: %w", err)
	}
	if mode != domain.PaymentModeDeposit && mode != domain.PaymentModeFull {
		mode = domain.PaymentModeDeposit
	}

	charge, err := s.computeCharge(ctx, req, mode)
	if err != nil {
		return nil, fmt.Errorf("operation: %w", err)
	}
	total := charge.totalPence
	currency := charge.currency
	paidInFull := mode == domain.PaymentModeFull

	isFree := false
	var reservedCodeID string
	hasMinor := HasMinor(req)
	var resp *domain.SubmitResult

	err = s.tx.WithTx(ctx, func(txCtx context.Context) error {
		freeCode := strings.ToUpper(strings.TrimSpace(req.FreeCode))
		if freeCode != "" {
			codeID, err := s.repo.ReserveFreeCode(txCtx, freeCode)
			if err != nil {
				if errors.Is(err, domain.ErrFreeCodeInvalid) {
					return ErrInvalidFreeCode
				}
				return fmt.Errorf("reserve free code: %w", err)
			}
			reservedCodeID = codeID
			isFree = true
			total = 0
			paidInFull = false
			charge.lines = nil
		}

		groupID, err := s.repo.InsertGroup(txCtx, req, total, currency, isFree, paidInFull)
		if err != nil {
			return fmt.Errorf("insert group: %w", err)
		}
		for _, c := range req.Campers {
			if err := s.repo.InsertCamper(txCtx, groupID, c); err != nil {
				return fmt.Errorf("insert camper: %w", err)
			}
		}
		if reservedCodeID != "" {
			if err := s.repo.MarkFreeCodeUsed(txCtx, reservedCodeID, groupID); err != nil {
				return fmt.Errorf("mark free code used: %w", err)
			}
		}

		resp = &domain.SubmitResult{
			GroupID:          groupID,
			TotalAmountPence: total,
			HasMinor:         hasMinor,
		}
		if hasMinor {
			resp.ConsentFormURL = s.publicBaseURL + "/api/consent-form"
		}

		if total == 0 {
			pending := domain.Group{
				PaymentStatus:            domain.PaymentPending,
				BillingStatus:            domain.BillingNone,
				PaidInFullAtRegistration: paidInFull,
			}
			settlement, _ := pending.SettleCheckout()
			coachIncluded := paidInFull && coachEligibleCountFromRequest(req) > 0
			if err := s.repo.ApplySettlement(txCtx, groupID, "", coachIncluded, settlement); err != nil {
				return fmt.Errorf("apply settlement: %w", err)
			}
			return nil
		}

		session, err := s.checkout.CreateCheckoutSession(txCtx, CheckoutParams{
			GroupID:  groupID,
			Email:    req.Contact.Email,
			Currency: currency,
			Lines:    charge.lines,
		})
		if err != nil {
			return fmt.Errorf("stripe checkout: %w", err)
		}
		if err := s.repo.SetCheckoutSession(txCtx, groupID, session.ID); err != nil {
			return fmt.Errorf("set stripe session: %w", err)
		}
		resp.CheckoutURL = session.URL
		return nil
	})
	if err != nil {
		return nil, err
	}

	if total == 0 {
		s.sendConfirmationEmail(ctx, req, total, currency, hasMinor, isFree, paidInFull)
		now := time.Now().UTC()
		rows := rowsFromRequest(resp.GroupID, req, domain.PaymentPaid, total, currency, now, &now)
		if err := s.sheets.AppendPaidAndRemovePending(ctx, resp.GroupID, rows); err != nil {
			log.Printf("sheets append paid (group %s): %v", resp.GroupID, err)
		}
		return resp, nil
	}

	pendingRows := rowsFromRequest(resp.GroupID, req, domain.PaymentPending, total, currency, time.Now().UTC(), nil)
	if err := s.sheets.AppendPending(ctx, pendingRows); err != nil {
		log.Printf("sheets append pending (group %s): %v", resp.GroupID, err)
	}
	return resp, nil
}

func rowsFromRequest(groupID string, req domain.Submission, status domain.PaymentStatus, totalPence int, currency string, submittedAt time.Time, paidAt *time.Time) []sheets.Row {
	rows := make([]sheets.Row, 0, len(req.Campers))
	for _, c := range req.Campers {
		row := sheets.Row{
			GroupID:          groupID,
			PaymentStatus:    string(status),
			SubmittedAt:      submittedAt,
			PaidAt:           paidAt,
			TotalAmountPence: totalPence,
			Currency:         currency,
			ContactFirstName: req.Contact.FirstName,
			ContactLastName:  req.Contact.LastName,
			ContactEmail:     req.Contact.Email,
			ContactPhone:     req.Contact.Phone,
			IsMainContact:    c.IsMainContact,
			FirstName:        c.FirstName,
			LastName:         c.LastName,
			Gender:           c.Gender,
			Age:              c.Age,
			CellLeaderName:   c.CellLeaderName,
			IsCellLeader:     c.IsCellLeader,
			AttendanceType:   c.Attendance.Type,
		}
		switch c.Attendance.Type {
		case domain.AttendanceFullWeek:
			row.ShirtSize = ptrIfNotEmpty(c.Attendance.ShirtSize)
			row.DietaryRequirements = ptrIfNotEmpty(c.Attendance.DietaryRequirements)
			row.NeedsCoach = c.Attendance.NeedsCoach
			row.AccommodationFirstChoice = ptrIfNotEmpty(c.Attendance.AccommodationFirstChoice)
			row.AccommodationSecondChoice = ptrIfNotEmpty(c.Attendance.AccommodationSecondChoice)
			row.RoommateRequests = ptrIfNotEmpty(c.Attendance.RoommateRequests)
		case domain.AttendanceDayPass:
			row.ShirtSize = ptrIfNotEmpty(c.Attendance.ShirtSize)
			row.DietaryRequirements = ptrIfNotEmpty(c.Attendance.DietaryRequirements)
			row.DayPassDays = c.Attendance.Days
			row.DayPassTshirtOption = ptrIfNotEmpty(c.Attendance.TshirtOption)
			row.DayPassNeedsCatering = c.Attendance.NeedsCatering
		}
		rows = append(rows, row)
	}
	return rows
}

func ptrIfNotEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// ensureAccommodationAvailable rejects submissions whose 1st/2nd accommodation
// choice points at a tier the White Team has disabled for registration. Only
// codes explicitly disabled are rejected; blank choices (day-pass campers) and
// unknown codes pass through unchanged.
func (s *Service) ensureAccommodationAvailable(ctx context.Context, req domain.Submission) error {
	avail, err := s.repo.AccommodationAvailability(ctx)
	if err != nil {
		return fmt.Errorf("accommodation availability: %w", err)
	}
	disabled := func(code string) bool {
		code = strings.TrimSpace(code)
		if code == "" {
			return false
		}
		ok, present := avail[code]
		return present && !ok
	}
	for _, c := range req.Campers {
		if disabled(c.Attendance.AccommodationFirstChoice) ||
			disabled(c.Attendance.AccommodationSecondChoice) {
			return ErrAccommodationUnavailable
		}
	}
	return nil
}

func (s *Service) computeTotal(ctx context.Context, req domain.Submission) (totalPence int, currency string, err error) {
	deposit, err := s.prices.GetPrice(ctx, domain.PriceDeposit)
	if err != nil {
		return 0, "", fmt.Errorf("lookup deposit price: %w", err)
	}
	currency = deposit.Currency
	if currency == "" {
		currency = "GBP"
	}
	totalPence = deposit.AmountPence * depositPayingCount(req)
	return totalPence, currency, nil
}

func (s *Service) sendConfirmationEmail(ctx context.Context, req domain.Submission, totalPence int, currency string, hasMinor, isFree, paidInFull bool) {
	if s.mailer == nil {
		return
	}
	consentURL := ""
	if hasMinor {
		consentURL = s.publicBaseURL + "/api/consent-form"
	}
	err := s.mailer.SendDepositConfirmation(ctx, email.DepositConfirmation{
		ToEmail:        req.Contact.Email,
		ToName:         req.Contact.FirstName,
		AmountPence:    totalPence,
		Currency:       currency,
		CamperCount:    len(req.Campers),
		HasMinor:       hasMinor,
		ConsentFormURL: consentURL,
		IsFree:         isFree,
		PaidInFull:     paidInFull,
	})
	if err != nil {
		log.Printf("send confirmation email to %s failed: %v", req.Contact.Email, err)
	}
}

func (s *Service) Summary(ctx context.Context, sessionID, groupID string) (*domain.RegistrationSummary, error) {
	if sessionID == "" && groupID == "" {
		return nil, ErrMissingIdentifier
	}

	var group *domain.Group
	var err error
	if sessionID != "" {
		group, err = s.repo.FindGroupBySessionID(ctx, sessionID)
	} else {
		group, err = s.repo.FindGroupByID(ctx, groupID)
	}
	if err != nil {
		return nil, fmt.Errorf("operation: %w", err)
	}
	if group == nil {
		return nil, nil
	}

	campers, err := s.repo.CampersForGroup(ctx, group.ID)
	if err != nil {
		return nil, fmt.Errorf("operation: %w", err)
	}
	out := &domain.RegistrationSummary{
		GroupID:          group.ID,
		PaymentStatus:    group.PaymentStatus,
		TotalAmountPence: group.TotalAmountPence,
		Currency:         group.Currency,
		ContactEmail:     group.ContactEmail,
		Campers:          make([]domain.SummaryCamper, 0, len(campers)),
	}
	for _, c := range campers {
		out.Campers = append(out.Campers, domain.SummaryCamper{
			FirstName: c.FirstName,
			LastName:  c.LastName,
		})
	}
	return out, nil
}

func depositPayingCount(req domain.Submission) int {
	n := 0
	for _, c := range req.Campers {
		if c.Attendance.Type == domain.AttendanceFullWeek && c.Age >= domain.MinDepositAge {
			n++
		}
	}
	return n
}

func pluralS(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

const freeCodePrefix = "SPON-"
const freeCodeCharset = "23456789ABCDEFGHJKLMNPQRSTUVWXYZ"

// GenerateFreeCode creates a one-time sponsorship code (admin only).
func (s *Service) GenerateFreeCode(ctx context.Context, actor, note string) (string, error) {
	for attempt := 0; attempt < 5; attempt++ {
		code, err := randomFreeCode()
		if err != nil {
			return "", fmt.Errorf("generate free code: %w", err)
		}
		if err := s.repo.GenerateFreeCode(ctx, code, actor, note); err != nil {
			if isUniqueViolation(err) {
				continue
			}
			return "", fmt.Errorf("generate free code: %w", err)
		}
		return code, nil
	}
	return "", errors.New("could not generate unique free code")
}

func randomFreeCode() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	out := make([]byte, 8)
	for i := range b {
		out[i] = freeCodeCharset[int(b[i])%len(freeCodeCharset)]
	}
	return freeCodePrefix + string(out), nil
}

func isUniqueViolation(err error) bool {
	return err != nil && strings.Contains(err.Error(), "duplicate key")
}

// ListFreeCodes returns all generated sponsorship codes with the code value
// masked. The full code is only ever returned once — at generation time, to
// the admin who created it — so a different admin can't read a code from the
// listing and redeem it before its intended recipient.
func (s *Service) ListFreeCodes(ctx context.Context) ([]domain.FreeCode, error) {
	codes, err := s.repo.ListFreeCodes(ctx)
	if err != nil {
		return nil, fmt.Errorf("operation: %w", err)
	}
	for i := range codes {
		codes[i].Code = MaskFreeCode(codes[i].Code)
	}
	return codes, nil
}

// MaskFreeCode hides everything but the last 4 characters, e.g.
// "SPON-Z9FEHB9Y" -> "*********HB9Y". The suffix is kept so the same code is
// still recognisable to whoever holds the full value, without revealing enough
// to redeem it. Used anywhere a code might be exposed to admins other than its
// creator (listings, audit log, etc.).
func MaskFreeCode(code string) string {
	const visible = 4
	r := []rune(code)
	if len(r) <= visible {
		return strings.Repeat("*", len(r))
	}
	return strings.Repeat("*", len(r)-visible) + string(r[len(r)-visible:])
}

// RevokeFreeCode disables an unused code.
func (s *Service) RevokeFreeCode(ctx context.Context, id string) error {
	if err := s.repo.RevokeFreeCode(ctx, id); err != nil {
		if errors.Is(err, domain.ErrFreeCodeNotRevocable) {
			return ErrCodeNotRevocable
		}
		return fmt.Errorf("revoke free code: %w", err)
	}
	return nil
}
