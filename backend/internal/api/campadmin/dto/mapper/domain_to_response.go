package mapper

import (
	"pagasacentre/backend/internal/adminlog"
	campdto "pagasacentre/backend/internal/api/camp/dto"
	campmapper "pagasacentre/backend/internal/api/camp/dto/mapper"
	"pagasacentre/backend/internal/api/campadmin/dto"
	"pagasacentre/backend/internal/billing/camperedit"
	campdomain "pagasacentre/backend/internal/camp/domain"
	"pagasacentre/backend/internal/registration/domain"
)

func CampConfigToResponse(cfg campdomain.Config) campdto.ConfigResponse {
	return campmapper.ConfigToResponse(cfg)
}

func GroupToResponse(g domain.Group, campers []domain.Camper) dto.GroupResponse {
	return dto.GroupResponse{
		ID:                       g.ID,
		ContactFirstName:         g.ContactFirstName,
		ContactLastName:          g.ContactLastName,
		ContactEmail:             g.ContactEmail,
		ContactPhone:             g.ContactPhone,
		PaymentStatus:            g.PaymentStatus,
		CheckoutSessionRef:       g.CheckoutSessionRef,
		DepositPaymentRef:        g.DepositPaymentRef,
		TotalAmountPence:         g.TotalAmountPence,
		Currency:                 g.Currency,
		CreatedAt:                g.CreatedAt,
		PaidAt:                   g.PaidAt,
		BillingCustomerRef:       g.BillingCustomerRef,
		BalanceInvoiceRef:        g.BalanceInvoiceRef,
		BillingStatus:            g.BillingStatus,
		InvoiceDueAt:             g.InvoiceDueAt,
		BalancePaidAt:            g.BalancePaidAt,
		Version:                  g.Version,
		LastAction:               g.LastAction,
		LastActionBy:             g.LastActionBy,
		LastActionAt:             g.LastActionAt,
		IsFree:                   g.IsFree,
		PaidInFullAtRegistration: g.PaidInFullAtRegistration,
		CoachIncludedInBalance:   g.CoachIncludedInBalance,
		CoachInvoiceRef:          g.CoachInvoiceRef,
		CoachInvoiceDueAt:        g.CoachInvoiceDueAt,
		CoachFeePaidAt:           g.CoachFeePaidAt,
		CoachFeeWaivedAt:         g.CoachFeeWaivedAt,
		Campers:                  CampersToResponse(campers),
	}
}

func CampersToResponse(campers []domain.Camper) []dto.CamperResponse {
	if campers == nil {
		return nil
	}
	out := make([]dto.CamperResponse, len(campers))
	for i, c := range campers {
		out[i] = CamperToResponse(c)
	}
	return out
}

func CamperToResponse(c domain.Camper) dto.CamperResponse {
	return dto.CamperResponse{
		ID:                         c.ID,
		GroupID:                    c.GroupID,
		IsMainContact:              c.IsMainContact,
		FirstName:                  c.FirstName,
		LastName:                   c.LastName,
		Gender:                     c.Gender,
		Age:                        c.Age,
		CellLeaderName:             c.CellLeaderName,
		IsCellLeader:               c.IsCellLeader,
		AttendanceType:             c.AttendanceType,
		ShirtSize:                  c.ShirtSize,
		DietaryRequirements:        c.DietaryRequirements,
		NeedsCoach:                 c.NeedsCoach,
		AccommodationFirstChoice:   c.AccommodationFirstChoice,
		AccommodationSecondChoice:  c.AccommodationSecondChoice,
		RoommateRequests:           c.RoommateRequests,
		DayPassDays:                c.DayPassDays,
		DayPassTshirtOption:        c.DayPassTshirtOption,
		DayPassNeedsCatering:       c.DayPassNeedsCatering,
		AllocatedAccommodationCode: c.AllocatedAccommodationCode,
		AllocatedUnitCode:          c.AllocatedUnitCode,
		BilledPriceRef:             c.BilledPriceRef,
		DepositCreditPence:         c.DepositCreditPence,
		DepositOwedPence:           c.DepositOwedPence,
		DepositPaidPence:           c.DepositPaidPence,
		CreatedAt:                  c.CreatedAt,
	}
}

func FreeCodesToResponse(codes []domain.FreeCode) []dto.FreeCodeResponse {
	if codes == nil {
		codes = []domain.FreeCode{}
	}
	out := make([]dto.FreeCodeResponse, len(codes))
	for i, fc := range codes {
		out[i] = dto.FreeCodeResponse{
			ID: fc.ID, Code: fc.Code, CreatedAt: fc.CreatedAt, CreatedBy: fc.CreatedBy,
			Note: fc.Note, UsedAt: fc.UsedAt, UsedByGroupID: fc.UsedByGroupID, RevokedAt: fc.RevokedAt,
		}
	}
	return out
}

func AccommodationTypesToResponse(types []domain.AccommodationType) []dto.AccommodationTypeResponse {
	if types == nil {
		return nil
	}
	out := make([]dto.AccommodationTypeResponse, len(types))
	for i, t := range types {
		out[i] = dto.AccommodationTypeResponse{
			Code: t.Code, DisplayName: t.DisplayName, Capacity: t.Capacity,
			PriceRef: t.PriceRef, AvailableForRegistration: t.AvailableForRegistration,
		}
	}
	return out
}

func AccommodationUnitsToResponse(units []domain.AccommodationUnit) []dto.AccommodationUnitResponse {
	if units == nil {
		return nil
	}
	out := make([]dto.AccommodationUnitResponse, len(units))
	for i, u := range units {
		out[i] = dto.AccommodationUnitResponse{
			Code: u.Code, AccommodationCode: u.AccommodationCode, DisplayName: u.DisplayName,
			Capacity: u.Capacity, SortOrder: u.SortOrder,
		}
	}
	return out
}

func EventsToResponse(events []adminlog.Event) []dto.EventResponse {
	if events == nil {
		events = []adminlog.Event{}
	}
	out := make([]dto.EventResponse, len(events))
	for i, e := range events {
		out[i] = EventToResponse(e)
	}
	return out
}

func EventToResponse(e adminlog.Event) dto.EventResponse {
	return dto.EventResponse{
		ID: e.ID, CreatedAt: e.CreatedAt, ActorName: e.ActorName, Action: e.Action,
		GroupID: e.GroupID, Summary: e.Summary, Metadata: e.Metadata,
	}
}

func EditCamperToResponse(s camperedit.EditCamperSummary) dto.EditCamperResponse {
	return dto.EditCamperResponse{
		CamperName: s.CamperName, PreviousName: s.PreviousName,
		InvoiceVoided: s.InvoiceVoided, Repriced: s.Repriced,
	}
}

func AddCamperToResponse(s camperedit.AddCamperSummary) dto.AddCamperResponse {
	return dto.AddCamperResponse{
		CamperID: s.CamperID, CamperName: s.CamperName, DepositOwedPence: s.DepositOwedPence,
		InvoiceVoided: s.InvoiceVoided, NeedsAllocation: s.NeedsAllocation,
	}
}
