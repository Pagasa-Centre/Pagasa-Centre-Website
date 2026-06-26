package registration

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"pagasacentre/backend/internal/email"
	"pagasacentre/backend/internal/registration/domain"
	"pagasacentre/backend/internal/registration/storage"
	"pagasacentre/backend/internal/sheets"
	commonerrors "pagasacentre/backend/pkg/commonlibrary/errors"
)

type PriceLookup interface {
	GetPrice(ctx context.Context, code string) (PriceRow, error)
}

type PriceRow struct {
	AmountPence int
	Currency    string
}

type CheckoutCreator interface {
	CreateCheckoutSession(ctx context.Context, p CheckoutParams) (CheckoutSession, error)
}

type CheckoutParams struct {
	GroupID     string
	Email       string
	AmountPence int64
	Currency    string
	Description string
}

type CheckoutSession struct {
	ID  string
	URL string
}

type CampConfigReader interface {
	RegistrationsOpen(ctx context.Context) (bool, error)
}

type Service struct {
	repo          *storage.Repository
	prices        PriceLookup
	stripe        CheckoutCreator
	camp          CampConfigReader
	mailer        email.Mailer
	sheets        sheets.Sync
	publicBaseURL string
}

func NewService(
	repo *storage.Repository,
	prices PriceLookup,
	stripe CheckoutCreator,
	campCfg CampConfigReader,
	mailer email.Mailer,
	sheetSync sheets.Sync,
	publicBaseURL string,
) *Service {
	if sheetSync == nil {
		sheetSync = sheets.NewNoopSync()
	}
	return &Service{
		repo:          repo,
		prices:        prices,
		stripe:        stripe,
		camp:          campCfg,
		mailer:        mailer,
		sheets:        sheetSync,
		publicBaseURL: publicBaseURL,
	}
}

func (s *Service) Submit(ctx context.Context, req domain.SubmitRequest) (*domain.SubmitResponse, error) {
	if err := Validate(req); err != nil {
		return nil, err
	}

	open, err := s.camp.RegistrationsOpen(ctx)
	if err != nil {
		return nil, commonerrors.Internal(err.Error())
	}
	if !open {
		return nil, commonerrors.APIError{
			Code:    "registrations_closed",
			Message: "Camp registrations are currently closed",
		}
	}

	total, currency, err := s.computeTotal(ctx, req)
	if err != nil {
		return nil, commonerrors.Internal(err.Error())
	}

	isFree := false
	var reservedCodeID string

	tx, err := s.repo.Pool().BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, commonerrors.Internal(err.Error())
	}
	defer func() { _ = tx.Rollback(ctx) }()

	freeCode := strings.ToUpper(strings.TrimSpace(req.FreeCode))
	if freeCode != "" {
		codeID, err := s.repo.ReserveFreeCode(ctx, tx, freeCode)
		if err != nil {
			if errors.Is(err, storage.ErrFreeCodeInvalid) {
				return nil, commonerrors.APIError{
					Code:    "invalid_free_code",
					Message: "That sponsorship code is not valid or has already been used.",
				}
			}
			return nil, commonerrors.Internal(err.Error())
		}
		reservedCodeID = codeID
		isFree = true
		total = 0
	}

	groupID, err := s.repo.InsertGroup(ctx, tx, req, total, currency, isFree)
	if err != nil {
		return nil, commonerrors.Internal(err.Error())
	}
	for _, c := range req.Campers {
		if err := s.repo.InsertCamper(ctx, tx, groupID, c); err != nil {
			return nil, commonerrors.Internal(err.Error())
		}
	}
	if reservedCodeID != "" {
		if err := s.repo.MarkFreeCodeUsed(ctx, tx, reservedCodeID, groupID); err != nil {
			return nil, commonerrors.Internal(err.Error())
		}
	}

	hasMinor := HasMinor(req)
	resp := &domain.SubmitResponse{
		GroupID:          groupID,
		TotalAmountPence: total,
		HasMinor:         hasMinor,
	}
	if hasMinor {
		resp.ConsentFormURL = s.publicBaseURL + "/api/consent-form"
	}

	if total == 0 {
		if err := s.repo.MarkPaid(ctx, tx, groupID, ""); err != nil {
			return nil, commonerrors.Internal(err.Error())
		}
		if err := tx.Commit(ctx); err != nil {
			return nil, commonerrors.Internal(err.Error())
		}
		s.sendConfirmationEmail(ctx, req, total, currency, hasMinor, isFree)
		now := time.Now().UTC()
		rows := rowsFromRequest(groupID, req, domain.PaymentPaid, total, currency, now, &now)
		if err := s.sheets.AppendPaidAndRemovePending(ctx, groupID, rows); err != nil {
			log.Printf("sheets append paid (group %s): %v", groupID, err)
		}
		return resp, nil
	}

	paying := depositPayingCount(req)
	session, err := s.stripe.CreateCheckoutSession(ctx, CheckoutParams{
		GroupID:     groupID,
		Email:       req.Contact.Email,
		AmountPence: int64(total),
		Currency:    currency,
		Description: fmt.Sprintf("PC Summer Camp 2026 non-refundable deposit (%d camper%s)", paying, pluralS(paying)),
	})
	if err != nil {
		return nil, commonerrors.Internal(fmt.Sprintf("stripe checkout: %s", err.Error()))
	}
	if err := s.repo.SetStripeSession(ctx, tx, groupID, session.ID); err != nil {
		return nil, commonerrors.Internal(err.Error())
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, commonerrors.Internal(err.Error())
	}

	pendingRows := rowsFromRequest(groupID, req, domain.PaymentPending, total, currency, time.Now().UTC(), nil)
	if err := s.sheets.AppendPending(ctx, pendingRows); err != nil {
		log.Printf("sheets append pending (group %s): %v", groupID, err)
	}

	resp.CheckoutURL = session.URL
	return resp, nil
}

func rowsFromRequest(groupID string, req domain.SubmitRequest, status string, totalPence int, currency string, submittedAt time.Time, paidAt *time.Time) []sheets.Row {
	rows := make([]sheets.Row, 0, len(req.Campers))
	for _, c := range req.Campers {
		row := sheets.Row{
			GroupID:          groupID,
			PaymentStatus:    status,
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

func (s *Service) computeTotal(ctx context.Context, req domain.SubmitRequest) (totalPence int, currency string, err error) {
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

func (s *Service) sendConfirmationEmail(ctx context.Context, req domain.SubmitRequest, totalPence int, currency string, hasMinor, isFree bool) {
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
	})
	if err != nil {
		log.Printf("send confirmation email to %s failed: %v", req.Contact.Email, err)
	}
}

const MinDepositAge = 4

func (s *Service) Summary(ctx context.Context, sessionID, groupID string) (*domain.SummaryResponse, error) {
	if sessionID == "" && groupID == "" {
		return nil, commonerrors.APIError{
			Code:    "missing_identifier",
			Message: "either session_id or group_id is required",
		}
	}

	var group *domain.Group
	var err error
	if sessionID != "" {
		group, err = s.repo.FindGroupBySessionID(ctx, sessionID)
	} else {
		group, err = s.repo.FindGroupByID(ctx, groupID)
	}
	if err != nil {
		return nil, commonerrors.Internal(err.Error())
	}
	if group == nil {
		return nil, nil
	}

	campers, err := s.repo.CampersForGroup(ctx, group.ID)
	if err != nil {
		return nil, commonerrors.Internal(err.Error())
	}
	out := &domain.SummaryResponse{
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

func depositPayingCount(req domain.SubmitRequest) int {
	n := 0
	for _, c := range req.Campers {
		if c.Attendance.Type == domain.AttendanceFullWeek && c.Age >= MinDepositAge {
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
			return "", commonerrors.Internal(err.Error())
		}
		if err := s.repo.GenerateFreeCode(ctx, code, actor, note); err != nil {
			if isUniqueViolation(err) {
				continue
			}
			return "", commonerrors.Internal(err.Error())
		}
		return code, nil
	}
	return "", commonerrors.Internal("could not generate unique free code")
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
		return nil, commonerrors.Internal(err.Error())
	}
	for i := range codes {
		codes[i].Code = maskFreeCode(codes[i].Code)
	}
	return codes, nil
}

// maskFreeCode hides everything but the last 4 characters, e.g.
// "SPON-Z9FEHB9Y" -> "*********HB9Y". The suffix is kept so the same code is
// still recognisable to whoever holds the full value, without revealing enough
// to redeem it.
func maskFreeCode(code string) string {
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
		if errors.Is(err, storage.ErrFreeCodeNotRevocable) {
			return commonerrors.BadRequest("code cannot be revoked (already used or revoked)", nil)
		}
		return commonerrors.Internal(err.Error())
	}
	return nil
}
