package reconcile

import (
	"context"
	"fmt"
	"log"
	"time"

	"pagasacentre/backend/internal/notify"
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
	if g.CoachFeeSettled() {
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
	coachCount := g.CoachCountOnBalance(campers)
	items := notify.BalanceItems(campers, accNames, s.pricer.UnitNames(ctx), coachCount)
	changes := notify.OffPreferenceChanges(campers, accNames)
	amount := notify.Amount{Pence: amountPaidPence, Currency: currency}

	if err := s.mailer.SendBalancePaidConfirmation(ctx, notify.BalancePaidConfirmation{
		ToEmail: g.ContactEmail,
		ToName:  g.ContactFirstName,
		Amount:  amount,
		Items:   items,
		Changes: changes,
	}); err != nil {
		log.Printf("billing: balance-paid confirmation email to %s: %v", g.ContactEmail, err)
	}

	if s.cfg.WhiteTeamEmail != "" {
		_ = s.mailer.SendBalancePaid(ctx, notify.BalancePaid{
			ToEmail:      s.cfg.WhiteTeamEmail,
			ContactName:  g.ContactName(),
			ContactEmail: g.ContactEmail,
			Amount:       amount,
			PaidAt:       time.Now(),
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
