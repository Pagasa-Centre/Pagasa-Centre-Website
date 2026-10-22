package campadmin

import (
	"context"

	"pagasacentre/backend/internal/adminlog"
)

func (s *Service) UpdateContact(ctx context.Context, groupID, actor, first, last, email, phone string, resend bool) error {
	if err := s.contact.UpdateGroupContact(ctx, groupID, first, last, email, phone, resend); err != nil {
		return err
	}
	gid := groupID
	summary := "Updated contact details for " + first + " " + last
	if resend {
		summary += " (confirmation email re-sent)"
	}
	s.record(ctx, actor, adminlog.ActionContactUpdated, &gid, summary, nil)
	return nil
}

func (s *Service) SetPaymentStatus(ctx context.Context, groupID, actor, status string) error {
	tr, err := s.reg.SetPaymentStatus(ctx, groupID, status)
	if err != nil {
		return err
	}
	if tr.From == tr.To {
		return nil
	}
	g, _ := s.reg.FindGroup(ctx, groupID)
	gid := groupID
	from := string(tr.From)
	to := string(tr.To)
	summary := "Changed payment status for " + groupSummary(g) + " from " + from + " to " + to
	s.record(ctx, actor, adminlog.ActionPaymentStatusUpdated, &gid, summary,
		map[string]string{"from": from, "to": to})
	return nil
}

func (s *Service) RecordLogin(ctx context.Context, name string) {
	if s == nil || s.rec == nil {
		return
	}
	s.record(ctx, name, adminlog.ActionLogin, nil, name+" signed in", nil)
}

func (s *Service) ListEvents(ctx context.Context, limit int, beforeID int64) ([]adminlog.Event, error) {
	if s == nil || s.rec == nil {
		return nil, nil
	}
	return s.rec.List(ctx, limit, beforeID)
}
