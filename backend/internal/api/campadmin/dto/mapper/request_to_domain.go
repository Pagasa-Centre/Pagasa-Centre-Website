package mapper

import (
	"pagasacentre/backend/internal/api/campadmin/dto"
	regmapper "pagasacentre/backend/internal/api/registration/dto/mapper"
	"pagasacentre/backend/internal/billing/allocation"
	"pagasacentre/backend/internal/billing/camperedit"
)

func AllocateToDomain(req dto.AllocateRequest) allocation.AllocateRequest {
	campers := make([]allocation.AllocateCamper, len(req.Campers))
	for i, c := range req.Campers {
		campers[i] = allocation.AllocateCamper{
			CamperID:                   c.CamperID,
			AllocatedAccommodationCode: c.AllocatedAccommodationCode,
			AllocatedUnitCode:          c.AllocatedUnitCode,
			BilledStripePriceID:        c.BilledStripePriceID,
		}
	}
	return allocation.AllocateRequest{Campers: campers}
}

func EditCamperToDomain(req dto.EditCamperRequest) camperedit.EditCamperRequest {
	return camperedit.EditCamperRequest{
		FirstName:                  req.FirstName,
		LastName:                   req.LastName,
		Gender:                     req.Gender,
		Age:                        req.Age,
		CellLeaderName:             req.CellLeaderName,
		IsCellLeader:               req.IsCellLeader,
		ShirtSize:                  req.ShirtSize,
		DietaryRequirements:        req.DietaryRequirements,
		AccommodationFirstChoice:   req.AccommodationFirstChoice,
		AccommodationSecondChoice:  req.AccommodationSecondChoice,
		AllocatedAccommodationCode: req.AllocatedAccommodationCode,
		AllocatedUnitCode:          req.AllocatedUnitCode,
	}
}

func AddCamperToDomain(req dto.AddCamperRequest) camperedit.AddCamperRequest {
	return camperedit.AddCamperRequest{Camper: regmapper.CamperToDomain(req.Camper)}
}

func ConvertToDayVisitorToDomain(req dto.ConvertToDayVisitorRequest) camperedit.ConvertToDayVisitorRequest {
	return camperedit.ConvertToDayVisitorRequest{
		Days:          req.Days,
		TshirtOption:  req.TshirtOption,
		ShirtSize:     req.ShirtSize,
		NeedsCatering: req.NeedsCatering,
	}
}

func UpdateDayPassToDomain(req dto.UpdateDayPassCamperRequest) camperedit.UpdateDayPassCamperRequest {
	return camperedit.UpdateDayPassCamperRequest{
		TshirtOption:  req.TshirtOption,
		ShirtSize:     req.ShirtSize,
		NeedsCatering: req.NeedsCatering,
		Dietary:       req.Dietary,
	}
}

func UpdateCoachToDomain(req dto.UpdateCamperCoachRequest) camperedit.UpdateCamperCoachRequest {
	return camperedit.UpdateCamperCoachRequest{NeedsCoach: req.NeedsCoach}
}
