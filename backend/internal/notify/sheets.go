package notify

import (
	"context"
	"log"
	"time"

	"pagasacentre/backend/internal/registration/domain"
)

type SheetSync interface {
	AppendPending(ctx context.Context, rows []SheetRow) error
	AppendPaidAndRemovePending(ctx context.Context, groupID string, rows []SheetRow) error
	UpdateContactByGroupID(ctx context.Context, groupID string, c ContactUpdate) error
	RemoveByGroupID(ctx context.Context, groupID string) error
}

type ContactUpdate struct {
	FirstName string
	LastName  string
	Email     string
	Phone     string
}

type SheetRow struct {
	GroupID                   string
	PaymentStatus             string
	SubmittedAt               time.Time
	PaidAt                    *time.Time
	TotalAmountPence          int
	Currency                  string
	ContactFirstName          string
	ContactLastName           string
	ContactEmail              string
	ContactPhone              string
	IsMainContact             bool
	FirstName                 string
	LastName                  string
	Gender                    string
	Age                       int
	CellLeaderName            string
	IsCellLeader              bool
	AttendanceType            string
	ShirtSize                 *string
	DietaryRequirements       *string
	NeedsCoach                *bool
	AccommodationFirstChoice  *string
	AccommodationSecondChoice *string
	RoommateRequests          *string
	DayPassDays               []string
	DayPassTshirtOption       *string
	DayPassNeedsCatering      *bool
}

func RowsForGroup(g *domain.Group, campers []domain.Camper) []SheetRow {
	rows := make([]SheetRow, 0, len(campers))
	for _, c := range campers {
		rows = append(rows, SheetRow{
			GroupID:                   g.ID,
			PaymentStatus:             string(g.PaymentStatus),
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

type NoopSheetSync struct{}

func NewNoopSheetSync() *NoopSheetSync { return &NoopSheetSync{} }

func (NoopSheetSync) AppendPending(_ context.Context, rows []SheetRow) error {
	if len(rows) == 0 {
		return nil
	}
	log.Printf("sheets noop: would append %d pending row(s) for group %s", len(rows), rows[0].GroupID)
	return nil
}

func (NoopSheetSync) AppendPaidAndRemovePending(_ context.Context, groupID string, rows []SheetRow) error {
	log.Printf("sheets noop: would move group %s to paid (%d row(s))", groupID, len(rows))
	return nil
}

func (NoopSheetSync) UpdateContactByGroupID(_ context.Context, groupID string, c ContactUpdate) error {
	log.Printf("sheets noop: would update contact for group %s (%s)", groupID, c.Email)
	return nil
}

func (NoopSheetSync) RemoveByGroupID(_ context.Context, groupID string) error {
	log.Printf("sheets noop: would remove rows for group %s", groupID)
	return nil
}
