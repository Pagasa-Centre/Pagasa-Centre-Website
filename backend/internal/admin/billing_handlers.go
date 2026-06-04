package admin

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"pagasacentre/backend/internal/billing"
	"pagasacentre/backend/internal/httpx"
	"pagasacentre/backend/internal/registration"
)

func putAllocation(svc *billing.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body billing.AllocateRequest
		if err := httpx.DecodeJSON(r, &body); err != nil {
			httpx.WriteError(w, err)
			return
		}
		groupID := chi.URLParam(r, "groupID")
		if err := svc.Allocate(r.Context(), groupID, body); err != nil {
			httpx.WriteError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func postInvoice(svc *billing.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		groupID := chi.URLParam(r, "groupID")
		if err := svc.SendInvoice(r.Context(), groupID); err != nil {
			httpx.WriteError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func postInvoiceBulk(svc *billing.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body billing.BulkInvoiceRequest
		if err := httpx.DecodeJSON(r, &body); err != nil {
			httpx.WriteError(w, err)
			return
		}
		errs := svc.SendInvoicesBulk(r.Context(), body.GroupIDs)
		if len(errs) > 0 {
			httpx.WriteJSON(w, http.StatusMultiStatus, map[string]any{"errors": errs})
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func postUnallocate(svc *billing.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		groupID := chi.URLParam(r, "groupID")
		if err := svc.Unallocate(r.Context(), groupID); err != nil {
			httpx.WriteError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func postRelease(svc *billing.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		groupID := chi.URLParam(r, "groupID")
		reason := "released by White Team"
		if err := svc.VoidAndRelease(r.Context(), groupID, reason); err != nil {
			httpx.WriteError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func postResendInvoice(svc *billing.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		groupID := chi.URLParam(r, "groupID")
		if err := svc.ResendInvoice(r.Context(), groupID); err != nil {
			httpx.WriteError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func patchInvoiceDue(svc *billing.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body billing.ExtendDueRequest
		if err := httpx.DecodeJSON(r, &body); err != nil {
			httpx.WriteError(w, err)
			return
		}
		dueAt, err := time.Parse(time.RFC3339, body.DueAt)
		if err != nil {
			httpx.WriteError(w, httpx.BadRequest("due_at must be RFC3339", nil))
			return
		}
		groupID := chi.URLParam(r, "groupID")
		if err := svc.ExtendDueAt(r.Context(), groupID, dueAt); err != nil {
			httpx.WriteError(w, httpx.Internal(err.Error()))
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func postBillingSweep(svc *billing.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		n, err := svc.SweepOverdue(r.Context())
		if err != nil {
			httpx.WriteError(w, httpx.Internal(err.Error()))
			return
		}
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"released": n})
	}
}

func listAccommodationTypes(repo accommodationLister) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		types, err := repo.ListAccommodationTypes(r.Context())
		if err != nil {
			httpx.WriteError(w, httpx.Internal(err.Error()))
			return
		}
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"accommodations": types})
	}
}

type accommodationLister interface {
	ListAccommodationTypes(context.Context) ([]registration.AccommodationType, error)
}
