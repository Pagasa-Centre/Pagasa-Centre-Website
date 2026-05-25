package payment

import (
	"context"
	"fmt"

	stripe "github.com/stripe/stripe-go/v79"
	checkoutsession "github.com/stripe/stripe-go/v79/checkout/session"
	"github.com/stripe/stripe-go/v79/webhook"

	"pagasacentre/backend/internal/registration"
)

// StripeClient wraps the stripe-go SDK calls we use. Implements
// registration.CheckoutCreator.
type StripeClient struct {
	secretKey     string
	webhookSecret string
	successURL    string
	cancelURL     string
}

func NewStripeClient(secretKey, webhookSecret, successURL, cancelURL string) *StripeClient {
	stripe.Key = secretKey
	return &StripeClient{
		secretKey:     secretKey,
		webhookSecret: webhookSecret,
		successURL:    successURL,
		cancelURL:     cancelURL,
	}
}

// CreateCheckoutSession satisfies registration.CheckoutCreator.
func (c *StripeClient) CreateCheckoutSession(ctx context.Context, p registration.CheckoutParams) (registration.CheckoutSession, error) {
	params := &stripe.CheckoutSessionParams{
		Mode:              stripe.String(string(stripe.CheckoutSessionModePayment)),
		SuccessURL:        stripe.String(c.successURL),
		CancelURL:         stripe.String(c.cancelURL),
		ClientReferenceID: stripe.String(p.GroupID),
		CustomerEmail:     stripe.String(p.Email),
		LineItems: []*stripe.CheckoutSessionLineItemParams{{
			Quantity: stripe.Int64(1),
			PriceData: &stripe.CheckoutSessionLineItemPriceDataParams{
				Currency:   stripe.String(p.Currency),
				UnitAmount: stripe.Int64(p.AmountPence),
				ProductData: &stripe.CheckoutSessionLineItemPriceDataProductDataParams{
					Name: stripe.String(p.Description),
				},
			},
		}},
		Metadata: map[string]string{"group_id": p.GroupID},
	}
	params.Context = ctx

	sess, err := checkoutsession.New(params)
	if err != nil {
		return registration.CheckoutSession{}, fmt.Errorf("stripe checkout.session.New: %w", err)
	}
	return registration.CheckoutSession{ID: sess.ID, URL: sess.URL}, nil
}

// VerifyWebhook validates a Stripe webhook signature and returns the parsed event.
//
// IgnoreAPIVersionMismatch is set because Stripe's default API version drifts
// faster than we bump stripe-go. The signature check is still performed; only
// the soft "this event was sent with version X but library was compiled
// against Y" error is suppressed. We only read top-level fields
// (event.Type / event.Data.Raw on checkout.session.*) which are stable across
// versions, so this is safe.
func (c *StripeClient) VerifyWebhook(body []byte, signature string) (stripe.Event, error) {
	return webhook.ConstructEventWithOptions(body, signature, c.webhookSecret, webhook.ConstructEventOptions{
		IgnoreAPIVersionMismatch: true,
	})
}
