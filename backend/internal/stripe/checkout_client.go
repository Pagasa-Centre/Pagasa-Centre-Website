package stripe

import (
	"context"
	"fmt"

	stripesdk "github.com/stripe/stripe-go/v85"
	checkoutsession "github.com/stripe/stripe-go/v85/checkout/session"
	"github.com/stripe/stripe-go/v85/webhook"

	"pagasacentre/backend/internal/registration"
)

type CheckoutClient struct {
	webhookSecret string
	successURL    string
	cancelURL     string
}

func NewCheckoutClient(secretKey, webhookSecret, successURL, cancelURL string) *CheckoutClient {
	_ = secretKey // key is set once via Init in main
	return &CheckoutClient{
		webhookSecret: webhookSecret,
		successURL:    successURL,
		cancelURL:     cancelURL,
	}
}

func (c *CheckoutClient) CreateCheckoutSession(ctx context.Context, p registration.CheckoutParams) (registration.CheckoutSession, error) {
	lineItems := make([]*stripesdk.CheckoutSessionLineItemParams, 0, len(p.Lines))
	for _, line := range p.Lines {
		if line.AmountPence <= 0 || line.Quantity < 1 {
			continue
		}
		lineItems = append(lineItems, &stripesdk.CheckoutSessionLineItemParams{
			Quantity: stripesdk.Int64(line.Quantity),
			PriceData: &stripesdk.CheckoutSessionLineItemPriceDataParams{
				Currency:   stripesdk.String(p.Currency),
				UnitAmount: stripesdk.Int64(line.AmountPence),
				ProductData: &stripesdk.CheckoutSessionLineItemPriceDataProductDataParams{
					Name: stripesdk.String(line.Description),
				},
			},
		})
	}
	if len(lineItems) == 0 {
		return registration.CheckoutSession{}, fmt.Errorf("checkout session requires at least one line item")
	}

	params := &stripesdk.CheckoutSessionParams{
		Mode:              stripesdk.String(string(stripesdk.CheckoutSessionModePayment)),
		SuccessURL:        stripesdk.String(c.successURL),
		CancelURL:         stripesdk.String(c.cancelURL),
		ClientReferenceID: stripesdk.String(p.GroupID),
		CustomerEmail:     stripesdk.String(p.Email),
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

func (c *CheckoutClient) VerifyWebhook(body []byte, signature string) (stripesdk.Event, error) {
	return webhook.ConstructEventWithOptions(body, signature, c.webhookSecret, webhook.ConstructEventOptions{
		IgnoreAPIVersionMismatch: true,
	})
}
