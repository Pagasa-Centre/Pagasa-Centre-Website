package testhelper

import (
	"time"

	accomdomain "pagasacentre/backend/internal/accommodation/domain"
	campdomain "pagasacentre/backend/internal/camp/domain"
	"pagasacentre/backend/internal/registration/domain"
)

func StrP(s string) *string        { return &s }
func BoolP(b bool) *bool           { return &b }
func IntP(n int) *int              { return &n }
func Int64P(n int64) *int64        { return &n }
func TimeP(t time.Time) *time.Time { return &t }

var FixedTime = time.Date(2026, 7, 15, 12, 30, 0, 0, time.UTC)

func SampleCampConfig() campdomain.Config {
	return campdomain.Config{
		Name:                    "Pag-Asa Camp 2026",
		LocationName:            "Ashburnham Place",
		LocationAddr:            "1 Main St",
		WebsiteURL:              "https://example.com",
		StartDate:               time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
		EndDate:                 time.Date(2026, 8, 8, 0, 0, 0, 0, time.UTC),
		RegistrationsOpen:       true,
		RegistrationPaymentMode: domain.PaymentModeDeposit,
	}
}

func SampleCampPrice() campdomain.Price {
	return campdomain.Price{
		Code:        domain.PriceDeposit,
		DisplayName: "Deposit",
		AmountPence: 5000,
		Currency:    "GBP",
	}
}

func SamplePublicAccommodationOption() accomdomain.Option {
	return accomdomain.Option{
		Code:                     "tent",
		DisplayName:              "Tent",
		Notes:                    "Bring your own",
		AvailableForRegistration: true,
	}
}

func SampleAccommodationType() domain.AccommodationType {
	cap := 40
	return domain.AccommodationType{
		Code:                     "tent",
		DisplayName:              "Tent",
		Capacity:                 &cap,
		PriceRef:                 StrP("price_123"),
		AvailableForRegistration: true,
	}
}

func SampleGroupFull() domain.Group {
	return domain.Group{
		ID:                       "grp-1",
		ContactFirstName:         "Jane",
		ContactLastName:          "Doe",
		ContactEmail:             "jane@example.com",
		ContactPhone:             "+441234567890",
		PaymentStatus:            domain.PaymentPaid,
		CheckoutSessionRef:       StrP("cs_test"),
		DepositPaymentRef:        StrP("pi_test"),
		TotalAmountPence:         10000,
		Currency:                 "GBP",
		CreatedAt:                FixedTime,
		PaidAt:                   TimeP(FixedTime),
		BillingCustomerRef:       StrP("cus_test"),
		BalanceInvoiceRef:        StrP("in_test"),
		BillingStatus:            domain.BillingInvoiced,
		InvoiceDueAt:             TimeP(FixedTime),
		BalancePaidAt:            TimeP(FixedTime),
		Version:                  3,
		LastAction:               StrP("invoice_sent"),
		LastActionBy:             StrP("admin"),
		LastActionAt:             TimeP(FixedTime),
		IsFree:                   false,
		PaidInFullAtRegistration: false,
		CoachIncludedInBalance:   true,
		CoachInvoiceRef:          StrP("in_coach"),
		CoachInvoiceDueAt:        TimeP(FixedTime),
		CoachFeePaidAt:           TimeP(FixedTime),
		CoachFeeWaivedAt:         TimeP(FixedTime),
	}
}

func SampleCamperFull() domain.Camper {
	return domain.Camper{
		ID:                         "cmp-1",
		GroupID:                    "grp-1",
		IsMainContact:              true,
		FirstName:                  "Alex",
		LastName:                   "Camper",
		Gender:                     domain.GenderMale,
		Age:                        25,
		CellLeaderName:             "Leader",
		IsCellLeader:               false,
		AttendanceType:             domain.AttendanceFullWeek,
		ShirtSize:                  StrP("adult_m"),
		DietaryRequirements:        StrP("none"),
		NeedsCoach:                 BoolP(true),
		AccommodationFirstChoice:   StrP("tent"),
		AccommodationSecondChoice:  StrP("caravan"),
		RoommateRequests:           StrP("friend"),
		DayPassDays:                []string{"mon", "tue"},
		DayPassTshirtOption:        StrP(domain.TshirtOptionTeamActivities),
		DayPassNeedsCatering:       BoolP(true),
		AllocatedAccommodationCode: StrP("tent"),
		AllocatedUnitCode:          StrP("tent-1"),
		BilledPriceRef:             StrP("price_abc"),
		DepositCreditPence:         100,
		DepositOwedPence:           200,
		DepositPaidPence:           300,
		CreatedAt:                  FixedTime,
	}
}

func SampleSubmitRequest() domain.Submission {
	needsCoach := true
	return domain.Submission{
		Contact: domain.Contact{
			FirstName: "Jane",
			LastName:  "Doe",
			Email:     "jane@example.com",
			Phone:     "+441234567890",
		},
		Campers: []domain.NewCamper{{
			FirstName:      "Alex",
			LastName:       "Camper",
			Gender:         domain.GenderMale,
			Age:            25,
			CellLeaderName: "Leader",
			IsCellLeader:   false,
			IsMainContact:  true,
			Attendance: domain.Attendance{
				Type:                      domain.AttendanceFullWeek,
				ShirtSize:                 "adult_m",
				DietaryRequirements:       "none",
				NeedsCoach:                &needsCoach,
				AccommodationFirstChoice:  "tent",
				AccommodationSecondChoice: "caravan",
				RoommateRequests:          "friend",
			},
		}},
		FreeCode: "FREE123",
	}
}

func SampleSubmitResponse() domain.SubmitResult {
	return domain.SubmitResult{
		GroupID:          "grp-1",
		CheckoutURL:      "https://checkout.stripe.com/test",
		TotalAmountPence: 5000,
		HasMinor:         true,
		ConsentFormURL:   "https://example.com/consent",
	}
}

func SampleRegistrationSummary() domain.RegistrationSummary {
	return domain.RegistrationSummary{
		GroupID:          "grp-1",
		PaymentStatus:    domain.PaymentPaid,
		TotalAmountPence: 10000,
		Currency:         "GBP",
		ContactEmail:     "jane@example.com",
		Campers: []domain.SummaryCamper{
			{FirstName: "Alex", LastName: "Camper"},
		},
	}
}

func SampleFreeCode() domain.FreeCode {
	return domain.FreeCode{
		ID:            "fc-1",
		Code:          "SPONSOR1",
		CreatedAt:     FixedTime,
		CreatedBy:     "admin",
		Note:          StrP("test note"),
		UsedAt:        TimeP(FixedTime),
		UsedByGroupID: StrP("grp-1"),
		RevokedAt:     TimeP(FixedTime),
	}
}

func SampleAccommodationUnit() domain.AccommodationUnit {
	return domain.AccommodationUnit{
		Code:              "tent-1",
		AccommodationCode: "tent",
		DisplayName:       "Tent 1",
		Capacity:          4,
		SortOrder:         1,
	}
}
