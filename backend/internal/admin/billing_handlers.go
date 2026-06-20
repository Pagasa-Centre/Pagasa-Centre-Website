package admin

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"pagasacentre/backend/internal/adminlog"
	"pagasacentre/backend/internal/billing"
	"pagasacentre/backend/internal/httpx"
	"pagasacentre/backend/internal/registration"
)

func putAllocation(svc *billing.Service, rec *adminlog.Recorder, regRepo *registration.Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body billing.AllocateRequest
		if err := httpx.DecodeJSON(r, &body); err != nil {
			httpx.WriteError(w, err)
			return
		}
		groupID := chi.URLParam(r, "groupID")
		actor := ActorFrom(r.Context())
		if err := svc.Allocate(r.Context(), groupID, actor, expectedVersion(body.ExpectedVersion), body); err != nil {
			httpx.WriteError(w, err)
			return
		}
		g, _ := regRepo.FindGroupByID(r.Context(), groupID)
		gid := groupID
		audit(rec, r, adminlog.ActionAllocate, &gid,
			"Allocated accommodation for "+groupSummary(g), nil)
		w.WriteHeader(http.StatusNoContent)
	}
}

func postInvoice(svc *billing.Service, rec *adminlog.Recorder, regRepo *registration.Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body billing.VersionedBody
		_ = httpx.DecodeJSON(r, &body) // body optional
		groupID := chi.URLParam(r, "groupID")
		actor := ActorFrom(r.Context())
		if err := svc.SendInvoice(r.Context(), groupID, actor, expectedVersion(body.ExpectedVersion)); err != nil {
			httpx.WriteError(w, err)
			return
		}
		g, _ := regRepo.FindGroupByID(r.Context(), groupID)
		gid := groupID
		audit(rec, r, adminlog.ActionInvoiceSent, &gid,
			"Sent balance invoice to "+groupSummary(g), nil)
		w.WriteHeader(http.StatusNoContent)
	}
}

func postInvoiceBulk(svc *billing.Service, rec *adminlog.Recorder, regRepo *registration.Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body billing.BulkInvoiceRequest
		if err := httpx.DecodeJSON(r, &body); err != nil {
			httpx.WriteError(w, err)
			return
		}
		actor := ActorFrom(r.Context())
		errs := svc.SendInvoicesBulk(r.Context(), actor, body.GroupIDs)
		for _, id := range body.GroupIDs {
			if _, failed := errs[id]; failed {
				continue
			}
			g, _ := regRepo.FindGroupByID(r.Context(), id)
			gid := id
			audit(rec, r, adminlog.ActionInvoiceSent, &gid,
				"Sent balance invoice to "+groupSummary(g), nil)
		}
		if len(errs) > 0 {
			httpx.WriteJSON(w, http.StatusMultiStatus, map[string]any{"errors": errs})
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func postUnallocate(svc *billing.Service, rec *adminlog.Recorder, regRepo *registration.Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body billing.VersionedBody
		_ = httpx.DecodeJSON(r, &body)
		groupID := chi.URLParam(r, "groupID")
		actor := ActorFrom(r.Context())
		if err := svc.Unallocate(r.Context(), groupID, actor, expectedVersion(body.ExpectedVersion)); err != nil {
			httpx.WriteError(w, err)
			return
		}
		g, _ := regRepo.FindGroupByID(r.Context(), groupID)
		gid := groupID
		audit(rec, r, adminlog.ActionUnallocate, &gid,
			"Reset allocation for "+groupSummary(g), nil)
		w.WriteHeader(http.StatusNoContent)
	}
}

func postRelease(svc *billing.Service, rec *adminlog.Recorder, regRepo *registration.Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body billing.VersionedBody
		_ = httpx.DecodeJSON(r, &body)
		groupID := chi.URLParam(r, "groupID")
		actor := ActorFrom(r.Context())
		reason := "released by White Team"
		if err := svc.VoidAndRelease(r.Context(), groupID, reason, actor, expectedVersion(body.ExpectedVersion)); err != nil {
			httpx.WriteError(w, err)
			return
		}
		g, _ := regRepo.FindGroupByID(r.Context(), groupID)
		gid := groupID
		audit(rec, r, adminlog.ActionRelease, &gid,
			"Released allocation for "+groupSummary(g), nil)
		w.WriteHeader(http.StatusNoContent)
	}
}

func postResendInvoice(svc *billing.Service, rec *adminlog.Recorder, regRepo *registration.Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body billing.VersionedBody
		_ = httpx.DecodeJSON(r, &body)
		groupID := chi.URLParam(r, "groupID")
		actor := ActorFrom(r.Context())
		if err := svc.ResendInvoice(r.Context(), groupID, actor, expectedVersion(body.ExpectedVersion)); err != nil {
			httpx.WriteError(w, err)
			return
		}
		g, _ := regRepo.FindGroupByID(r.Context(), groupID)
		gid := groupID
		audit(rec, r, adminlog.ActionInvoiceResent, &gid,
			"Re-sent invoice reminder to "+groupSummary(g), nil)
		w.WriteHeader(http.StatusNoContent)
	}
}

func patchInvoiceDue(svc *billing.Service, rec *adminlog.Recorder, regRepo *registration.Repository) http.HandlerFunc {
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
		actor := ActorFrom(r.Context())
		if err := svc.ExtendDueAt(r.Context(), groupID, actor, expectedVersion(body.ExpectedVersion), dueAt); err != nil {
			httpx.WriteError(w, err)
			return
		}
		g, _ := regRepo.FindGroupByID(r.Context(), groupID)
		gid := groupID
		audit(rec, r, adminlog.ActionExtendDue, &gid,
			"Extended invoice due date for "+groupSummary(g), nil)
		w.WriteHeader(http.StatusNoContent)
	}
}

func postBillingSweep(svc *billing.Service, rec *adminlog.Recorder) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		n, err := svc.SweepOverdue(r.Context())
		if err != nil {
			httpx.WriteError(w, httpx.Internal(err.Error()))
			return
		}
		audit(rec, r, adminlog.ActionSweep, nil,
			strings.TrimSpace(ActorFrom(r.Context()))+" released "+strconv.Itoa(n)+" overdue group(s)", map[string]int{"released": n})
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

func listAccommodationUnits(repo unitsLister) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		units, err := repo.ListAccommodationUnits(r.Context())
		if err != nil {
			httpx.WriteError(w, httpx.Internal(err.Error()))
			return
		}
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"units": units})
	}
}

type accommodationLister interface {
	ListAccommodationTypes(context.Context) ([]registration.AccommodationType, error)
}

type unitsLister interface {
	ListAccommodationUnits(context.Context) ([]registration.AccommodationUnit, error)
}
