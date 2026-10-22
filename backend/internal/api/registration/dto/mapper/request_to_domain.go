package mapper

import (
	"pagasacentre/backend/internal/api/registration/dto"
	"pagasacentre/backend/internal/registration/domain"
)

func RequestToDomain(req dto.SubmitRequest) domain.Submission {
	campers := make([]domain.NewCamper, len(req.Campers))
	for i, c := range req.Campers {
		campers[i] = CamperToDomain(c)
	}
	return domain.Submission{
		Contact: domain.Contact{
			FirstName: req.Contact.FirstName,
			LastName:  req.Contact.LastName,
			Email:     req.Contact.Email,
			Phone:     req.Contact.Phone,
		},
		Campers:  campers,
		FreeCode: req.FreeCode,
	}
}

func SubmissionToRequest(s domain.Submission) dto.SubmitRequest {
	campers := make([]dto.CamperRequest, len(s.Campers))
	for i, c := range s.Campers {
		campers[i] = CamperToRequest(c)
	}
	return dto.SubmitRequest{
		Contact: dto.ContactRequest{
			FirstName: s.Contact.FirstName,
			LastName:  s.Contact.LastName,
			Email:     s.Contact.Email,
			Phone:     s.Contact.Phone,
		},
		Campers:  campers,
		FreeCode: s.FreeCode,
	}
}

func CamperToRequest(c domain.NewCamper) dto.CamperRequest {
	return dto.CamperRequest{
		FirstName: c.FirstName, LastName: c.LastName, Gender: c.Gender, Age: c.Age,
		CellLeaderName: c.CellLeaderName, IsCellLeader: c.IsCellLeader, IsMainContact: c.IsMainContact,
		Attendance: dto.AttendanceRequest{
			Type: c.Attendance.Type, ShirtSize: c.Attendance.ShirtSize,
			DietaryRequirements: c.Attendance.DietaryRequirements, NeedsCoach: c.Attendance.NeedsCoach,
			AccommodationFirstChoice:  c.Attendance.AccommodationFirstChoice,
			AccommodationSecondChoice: c.Attendance.AccommodationSecondChoice,
			RoommateRequests:          c.Attendance.RoommateRequests, Days: c.Attendance.Days,
			TshirtOption: c.Attendance.TshirtOption, NeedsCatering: c.Attendance.NeedsCatering,
		},
	}
}

func CamperToDomain(c dto.CamperRequest) domain.NewCamper {
	return domain.NewCamper{
		FirstName:      c.FirstName,
		LastName:       c.LastName,
		Gender:         c.Gender,
		Age:            c.Age,
		CellLeaderName: c.CellLeaderName,
		IsCellLeader:   c.IsCellLeader,
		IsMainContact:  c.IsMainContact,
		Attendance: domain.Attendance{
			Type:                      c.Attendance.Type,
			ShirtSize:                 c.Attendance.ShirtSize,
			DietaryRequirements:       c.Attendance.DietaryRequirements,
			NeedsCoach:                c.Attendance.NeedsCoach,
			AccommodationFirstChoice:  c.Attendance.AccommodationFirstChoice,
			AccommodationSecondChoice: c.Attendance.AccommodationSecondChoice,
			RoommateRequests:          c.Attendance.RoommateRequests,
			Days:                      c.Attendance.Days,
			TshirtOption:              c.Attendance.TshirtOption,
			NeedsCatering:             c.Attendance.NeedsCatering,
		},
	}
}
