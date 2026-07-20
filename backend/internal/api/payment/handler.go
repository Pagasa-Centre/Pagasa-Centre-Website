package payment

import (
	"encoding/json"
	"io"
	"log"
	"net/http"

	stripe "github.com/stripe/stripe-go/v85"

	"pagasacentre/backend/internal/billing"
	paysvc "pagasacentre/backend/internal/payment"
	"pagasacentre/backend/pkg/commonlibrary/render"
)

type WebhookVerifier interface {
	VerifyWebhook(body []byte, signature string) (stripe.Event, error)
}

type Handler struct {
	service   *paysvc.Service
	billSvc   *billing.Service
	verifier  WebhookVerifier
}

func NewHandler(service *paysvc.Service, billSvc *billing.Service, verifier WebhookVerifier) *Handler {
	return &Handler{service: service, billSvc: billSvc, verifier: verifier}
}

func (h *Handler) Webhook() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
		if err != nil {
			http.Error(w, "read body", http.StatusBadRequest)
			return
		}
		event, err := h.verifier.VerifyWebhook(body, r.Header.Get("Stripe-Signature"))
		if err != nil {
			log.Printf("stripe webhook signature verification failed: %v", err)
			http.Error(w, "invalid signature", http.StatusBadRequest)
			return
		}

		ctx := r.Context()
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
			if err := h.service.HandleCheckoutCompleted(ctx, paysvc.CheckoutCompleted{
				SessionID:       sess.ID,
				PaymentIntentID: pi,
			}); err != nil {
				log.Printf("handle checkout.session.completed: %v", err)
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
			if err := h.service.HandleCheckoutExpired(ctx, sess.ID); err != nil {
				log.Printf("handle checkout.session.expired: %v", err)
				http.Error(w, "internal", http.StatusInternalServerError)
				return
			}
		case "invoice.paid":
			if h.billSvc == nil {
				break
			}
			var inv stripe.Invoice
			if err := json.Unmarshal(event.Data.Raw, &inv); err != nil {
				log.Printf("decode invoice: %v", err)
				http.Error(w, "decode payload", http.StatusBadRequest)
				return
			}
			groupID := inv.Metadata["group_id"]
			if inv.Metadata["invoice_type"] == "coach" {
				if err := h.billSvc.HandleCoachInvoicePaid(ctx, inv.ID, groupID); err != nil {
					log.Printf("handle coach invoice.paid: %v", err)
					http.Error(w, "internal", http.StatusInternalServerError)
					return
				}
				break
			}
			if err := h.billSvc.HandleInvoicePaid(ctx, inv.ID, groupID, inv.AmountPaid, string(inv.Currency)); err != nil {
				log.Printf("handle invoice.paid: %v", err)
				http.Error(w, "internal", http.StatusInternalServerError)
				return
			}
		case "invoice.payment_failed":
			if h.billSvc != nil {
				var inv stripe.Invoice
				if err := json.Unmarshal(event.Data.Raw, &inv); err == nil {
					h.billSvc.HandleInvoiceFailed(ctx, inv.Metadata["group_id"])
				}
			}
		case "invoice.marked_uncollectible":
			if h.billSvc != nil {
				var inv stripe.Invoice
				if err := json.Unmarshal(event.Data.Raw, &inv); err == nil {
					h.billSvc.HandleInvoiceUncollectible(ctx, inv.Metadata["group_id"])
				}
			}
		default:
		}
		render.Json(w, http.StatusOK, map[string]string{"received": "ok"})
	}
}
