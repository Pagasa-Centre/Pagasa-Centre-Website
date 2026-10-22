package reconcile

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"pagasacentre/backend/internal/billing"
	"pagasacentre/backend/internal/email"
	"pagasacentre/backend/internal/registration/domain"
)

func (s *Service) HandleCoachInvoicePaid(ctx context.Context, stripeInvoiceID, groupID string) error {
	g, err := s.reads.FindGroupByID(ctx, groupID)
	if err != nil {
		return err
	}
	if g == nil {
		g, err = s.repo.FindGroupByCoachInvoiceRef(ctx, stripeInvoiceID)
		if err != nil || g == nil {
			return fmt.Errorf("group not found for coach invoice %s", stripeInvoiceID)
		}
	}
	if g.CoachFeePaidAt != nil {
		return nil
	}
	return s.repo.MarkCoachFeePaidMeta(ctx, g.ID, domain.ActionMeta{
		Actor:           "Stripe",
		Action:          "coach_fee_paid",
		ExpectedVersion: domain.SkipVersionCheck,
	})
}

func (s *Service) HandleInvoicePaid(ctx context.Context, stripeInvoiceID, groupID string, amountPaidPence int64, currency string) error {
	g, err := s.reads.FindGroupByID(ctx, groupID)
	if err != nil {
		return err
	}
	if g == nil {
		g, err = s.repo.FindGroupByBalanceInvoiceRef(ctx, stripeInvoiceID)
		if err != nil || g == nil {
			return fmt.Errorf("group not found for invoice %s", stripeInvoiceID)
		}
	}
	tr, already := g.SettleBalance()
	if already {
		return nil
	}
	if err := s.repo.MarkBalancePaidMeta(ctx, g.ID, domain.ActionMeta{
		Actor:           "Stripe",
		Action:          "balance_paid",
		ExpectedVersion: domain.SkipVersionCheck,
	}, tr); err != nil {
		return err
	}
	campers, _ := s.reads.CampersForGroup(ctx, g.ID)
	accNames := s.pricer.AccommodationNames(ctx)
	coachCount := int64(0)
	if g.CoachIncludedInBalance {
		coachCount = billing.CoachEligibleCount(campers)
	}
	items := billing.BalanceInvoiceItems(campers, accNames, s.pricer.UnitNames(ctx), coachCount)
	changes := billing.OffPreferenceChanges(campers, accNames)
	amountLabel := billing.FormatAmountLabel(amountPaidPence, currency)

	// Confirm to the family that their place is fully paid and confirmed.
	if err := s.mailer.SendBalancePaidConfirmation(ctx, email.BalancePaidConfirmation{
		ToEmail:     g.ContactEmail,
		ToName:      g.ContactFirstName,
		AmountLabel: amountLabel,
		Items:       items,
		Changes:     changes,
	}); err != nil {
		log.Printf("billing: balance-paid confirmation email to %s: %v", g.ContactEmail, err)
	}

	if s.cfg.WhiteTeamEmail != "" {
		_ = s.mailer.SendBalancePaid(ctx, email.BalancePaid{
			ToEmail:      s.cfg.WhiteTeamEmail,
			ContactName:  strings.TrimSpace(g.ContactFirstName + " " + g.ContactLastName),
			ContactEmail: g.ContactEmail,
			AmountLabel:  amountLabel,
			PaidDate:     time.Now().Format("2 Jan 2006"),
			Items:        items,
		})
	}
	return nil
}

func (s *Service) HandleInvoiceFailed(ctx context.Context, groupID string) {
	log.Printf("billing: invoice payment failed for group %s", groupID)
}

func (s *Service) HandleInvoiceUncollectible(ctx context.Context, groupID string) {
	log.Printf("billing: invoice marked uncollectible for group %s", groupID)
}
