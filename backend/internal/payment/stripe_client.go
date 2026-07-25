package payment

import (
	"context"
	"fmt"

	stripe "github.com/stripe/stripe-go/v85"
	checkoutsession "github.com/stripe/stripe-go/v85/checkout/session"
	"github.com/stripe/stripe-go/v85/webhook"

	"pagasacentre/backend/internal/registration"
)

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

func (c *StripeClient) CreateCheckoutSession(ctx context.Context, p registration.CheckoutParams) (registration.CheckoutSession, error) {
	lineItems := make([]*stripe.CheckoutSessionLineItemParams, 0, len(p.Lines))
	for _, line := range p.Lines {
		if line.AmountPence <= 0 || line.Quantity < 1 {
			continue
		}
		lineItems = append(lineItems, &stripe.CheckoutSessionLineItemParams{
			Quantity: stripe.Int64(line.Quantity),
			PriceData: &stripe.CheckoutSessionLineItemPriceDataParams{
				Currency:   stripe.String(p.Currency),
				UnitAmount: stripe.Int64(line.AmountPence),
				ProductData: &stripe.CheckoutSessionLineItemPriceDataProductDataParams{
					Name: stripe.String(line.Description),
				},
			},
		})
	}
	if len(lineItems) == 0 {
		return registration.CheckoutSession{}, fmt.Errorf("checkout session requires at least one line item")
	}

	params := &stripe.CheckoutSessionParams{
		Mode:              stripe.String(string(stripe.CheckoutSessionModePayment)),
		SuccessURL:        stripe.String(c.successURL),
		CancelURL:         stripe.String(c.cancelURL),
		ClientReferenceID: stripe.String(p.GroupID),
		CustomerEmail:     stripe.String(p.Email),
		LineItems:         lineItems,
		Metadata:          map[string]string{"group_id": p.GroupID},
	}
	params.Context = ctx

	sess, err := checkoutsession.New(params)
	if err != nil {
		return registration.CheckoutSession{}, fmt.Errorf("stripe checkout.session.New: %w", err)
	}
	return registration.CheckoutSession{ID: sess.ID, URL: sess.URL}, nil
}

func (c *StripeClient) VerifyWebhook(body []byte, signature string) (stripe.Event, error) {
	return webhook.ConstructEventWithOptions(body, signature, c.webhookSecret, webhook.ConstructEventOptions{
		IgnoreAPIVersionMismatch: true,
	})
}
