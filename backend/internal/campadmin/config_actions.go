package campadmin

import (
	"context"

	"pagasacentre/backend/internal/adminlog"
	"pagasacentre/backend/internal/registration"
	"pagasacentre/backend/internal/registration/domain"
)

func (s *Service) SetRegistrationsOpen(ctx context.Context, actor string, open bool) error {
	if err := s.camp.SetRegistrationsOpen(ctx, open); err != nil {
		return err
	}
	state := "closed"
	if open {
		state = "opened"
	}
	s.record(ctx, actor, adminlog.ActionRegistrationsToggle, nil,
		actor+" "+state+" public registration", map[string]bool{"open": open})
	return nil
}

func (s *Service) SetRegistrationPaymentMode(ctx context.Context, actor, mode string) error {
	if mode == domain.PaymentModeFull {
		if err := s.reg.ValidateFullPaymentPricing(ctx); err != nil {
			return err
		}
	}
	if err := s.camp.SetRegistrationPaymentMode(ctx, mode); err != nil {
		return err
	}
	label := "deposit mode"
	if mode == domain.PaymentModeFull {
		label = "full payment mode"
	}
	s.record(ctx, actor, adminlog.ActionPaymentModeToggle, nil,
		actor+" switched registration to "+label, map[string]string{"mode": mode})
	return nil
}

func (s *Service) UpdatePrice(ctx context.Context, actor, code string, amountPence int) error {
	if err := s.camp.UpdatePrice(ctx, code, amountPence); err != nil {
		return err
	}
	s.record(ctx, actor, adminlog.ActionPriceUpdated, nil,
		actor+" updated price "+code, map[string]any{"code": code, "amount_pence": amountPence})
	return nil
}

func (s *Service) SetAccommodationAvailability(ctx context.Context, actor, code string, available bool) error {
	if err := s.reg.SetAccommodationAvailability(ctx, code, available); err != nil {
		return err
	}
	state := "hid"
	if available {
		state = "showed"
	}
	s.record(ctx, actor, adminlog.ActionAccommodationAvailability, nil,
		actor+" "+state+" "+code+" on the registration form",
		map[string]any{"code": code, "available_for_registration": available})
	return nil
}

func (s *Service) GenerateFreeCode(ctx context.Context, actor, note string) (string, error) {
	code, err := s.reg.GenerateFreeCode(ctx, actor, note)
	if err != nil {
		return "", err
	}
	masked := registration.MaskFreeCode(code)
	s.record(ctx, actor, adminlog.ActionFreeCodeGenerated, nil,
		actor+" generated sponsorship code "+masked, map[string]string{"code": masked})
	return code, nil
}

func (s *Service) RevokeFreeCode(ctx context.Context, actor, id string) error {
	if err := s.reg.RevokeFreeCode(ctx, id); err != nil {
		return err
	}
	s.record(ctx, actor, adminlog.ActionFreeCodeRevoked, nil,
		actor+" revoked sponsorship code "+id, map[string]string{"code_id": id})
	return nil
}
