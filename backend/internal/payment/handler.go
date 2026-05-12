package payment

import (
	"encoding/json"
	"io"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	stripe "github.com/stripe/stripe-go/v79"

	"pagasacentre/backend/internal/httpx"
)

// WebhookVerifier is implemented by *StripeClient. Defined here so we can mock
// it in tests.
type WebhookVerifier interface {
	VerifyWebhook(body []byte, signature string) (stripe.Event, error)
}

// Mount registers POST /payments/webhook.
func Mount(r chi.Router, svc *Service, verifier WebhookVerifier) {
	r.Post("/payments/webhook", func(w http.ResponseWriter, req *http.Request) {
		body, err := io.ReadAll(http.MaxBytesReader(w, req.Body, 1<<20))
		if err != nil {
			http.Error(w, "read body", http.StatusBadRequest)
			return
		}
		event, err := verifier.VerifyWebhook(body, req.Header.Get("Stripe-Signature"))
		if err != nil {
			log.Printf("stripe webhook signature verification failed: %v", err)
			http.Error(w, "invalid signature", http.StatusBadRequest)
			return
		}

		ctx := req.Context()
		switch event.Type {
		case "checkout.session.completed":
			var sess stripe.CheckoutSession
			if err := json.Unmarshal(event.Data.Raw, &sess); err != nil {
				log.Printf("decode session: %v", err)
				http.Error(w, "decode payload", http.StatusBadRequest)
				return
			}
			pi := ""
			if sess.PaymentIntent != nil {
				pi = sess.PaymentIntent.ID
			}
			if err := svc.HandleCheckoutCompleted(ctx, CheckoutCompleted{
				SessionID:       sess.ID,
				PaymentIntentID: pi,
			}); err != nil {
				log.Printf("handle checkout.session.completed: %v", err)
				// Returning 500 makes Stripe retry, which is what we want for
				// transient DB errors.
				http.Error(w, "internal", http.StatusInternalServerError)
				return
			}
		case "checkout.session.expired":
			var sess stripe.CheckoutSession
			if err := json.Unmarshal(event.Data.Raw, &sess); err != nil {
				log.Printf("decode session: %v", err)
				http.Error(w, "decode payload", http.StatusBadRequest)
				return
			}
			if err := svc.HandleCheckoutExpired(ctx, sess.ID); err != nil {
				log.Printf("handle checkout.session.expired: %v", err)
				http.Error(w, "internal", http.StatusInternalServerError)
				return
			}
		default:
			// Ignore other event types; respond 200 so Stripe doesn't retry.
		}
		httpx.WriteJSON(w, http.StatusOK, map[string]string{"received": "ok"})
	})
}
