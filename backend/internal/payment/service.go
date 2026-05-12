package payment

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"pagasacentre/backend/internal/accommodation"
	"pagasacentre/backend/internal/registration"
)

// AccommodationLocker is the slim interface the service needs from the
// accommodation repository so the webhook can lock + recount within a tx.
type AccommodationLocker interface {
	LockAndCount(ctx context.Context, tx pgx.Tx, code string) (*int, int, error)
}

// Refunder is the surface the service needs from the Stripe client to issue
// race-loser refunds.
type Refunder interface {
	Refund(ctx context.Context, paymentIntentID string) error
}

// Service handles Stripe webhook events.
type Service struct {
	pool    *pgxpool.Pool
	regRepo *registration.Repository
	accRepo AccommodationLocker
	refund  Refunder
}

func NewService(pool *pgxpool.Pool, regRepo *registration.Repository, accRepo AccommodationLocker, refund Refunder) *Service {
	return &Service{pool: pool, regRepo: regRepo, accRepo: accRepo, refund: refund}
}

// CheckoutCompleted is the payload extracted from a checkout.session.completed
// event.
type CheckoutCompleted struct {
	SessionID       string
	PaymentIntentID string
}

// HandleCheckoutCompleted is the authoritative step that transitions a group
// to 'paid' and reserves accommodation slots. If capacity has been exhausted
// by a concurrent winner, the group is marked failed_capacity and a refund is
// issued.
func (s *Service) HandleCheckoutCompleted(ctx context.Context, evt CheckoutCompleted) error {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback(ctx)
		}
	}()

	group, err := s.regRepo.GetGroupBySession(ctx, tx, evt.SessionID)
	if err != nil {
		return err
	}
	if group == nil {
		return fmt.Errorf("no registration group for session %s", evt.SessionID)
	}
	if group.PaymentStatus != registration.PaymentPending {
		return nil // idempotent
	}

	counts, err := s.regRepo.AccommodationCountsForGroup(ctx, tx, group.ID)
	if err != nil {
		return err
	}

	overflowing := false
	for code, needed := range counts {
		capacity, taken, err := s.accRepo.LockAndCount(ctx, tx, code)
		if err != nil {
			return err
		}
		if capacity != nil && taken+needed > *capacity {
			overflowing = true
			break
		}
	}

	if overflowing {
		if err := s.regRepo.MarkFailedCapacity(ctx, tx, group.ID, evt.PaymentIntentID); err != nil {
			return err
		}
		if err := tx.Commit(ctx); err != nil {
			return err
		}
		committed = true
		if refundErr := s.refund.Refund(ctx, evt.PaymentIntentID); refundErr != nil {
			return fmt.Errorf("mark failed_capacity ok, refund failed: %w", refundErr)
		}
		return nil
	}

	if err := s.regRepo.MarkPaid(ctx, tx, group.ID, evt.PaymentIntentID); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	committed = true
	return nil
}

// HandleCheckoutExpired marks a group as cancelled when the Stripe session
// expires without payment.
func (s *Service) HandleCheckoutExpired(ctx context.Context, sessionID string) error {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	group, err := s.regRepo.GetGroupBySession(ctx, tx, sessionID)
	if err != nil {
		return err
	}
	if group == nil || group.PaymentStatus != registration.PaymentPending {
		return nil
	}
	if err := s.regRepo.MarkStatusInTx(ctx, tx, group.ID, registration.PaymentCancelled); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// Ensure accommodation.Repository satisfies AccommodationLocker via a small
// adapter; this also documents the dependency direction.
var _ AccommodationLocker = (*accommodationLockerAdapter)(nil)

type accommodationLockerAdapter struct{ repo *accommodation.Repository }

func (a accommodationLockerAdapter) LockAndCount(ctx context.Context, tx pgx.Tx, code string) (*int, int, error) {
	return a.repo.LockAndCount(ctx, tx, code)
}

// NewAccommodationLocker wraps an *accommodation.Repository so it satisfies
// AccommodationLocker.
func NewAccommodationLocker(repo *accommodation.Repository) AccommodationLocker {
	return accommodationLockerAdapter{repo: repo}
}

// ErrUnhandledEvent is returned by the handler for event types it ignores.
var ErrUnhandledEvent = errors.New("unhandled event type")
