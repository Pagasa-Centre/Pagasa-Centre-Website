package admin

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"pagasacentre/backend/internal/admin"
	"pagasacentre/backend/internal/adminlog"
	"pagasacentre/backend/internal/billing"
	"pagasacentre/backend/internal/registration/domain"
	regstorage "pagasacentre/backend/internal/registration/storage"
	commonerrors "pagasacentre/backend/pkg/commonlibrary/errors"
	"pagasacentre/backend/pkg/commonlibrary/render"
	"pagasacentre/backend/pkg/commonlibrary/request"
)

func putAllocation(svc *billing.Service, rec *adminlog.Recorder, regRepo *regstorage.Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body billing.AllocateRequest
		if err := request.Decode(r, &body); err != nil {
			commonerrors.WriteError(w, err)
			return
		}
		groupID := chi.URLParam(r, "groupID")
		actor := admin.ActorFrom(r.Context())

		// Capture the prior billing status so we can distinguish a first-time
		// allocation ("none") from an edit of an existing allocation
		// ("allocated"). This makes the activity log show who did the initial
		// placement vs. who later changed it.
		prior, _ := regRepo.FindGroupByID(r.Context(), groupID)
		isEdit := prior != nil && prior.BillingStatus == domain.BillingAllocated

		if err := svc.Allocate(r.Context(), groupID, actor, billing.ExpectedVersion(body.ExpectedVersion), body); err != nil {
			commonerrors.WriteError(w, err)
			return
		}
		g, _ := regRepo.FindGroupByID(r.Context(), groupID)
		gid := groupID
		if isEdit {
			admin.Audit(rec, r, adminlog.ActionAllocationEdited, &gid,
				"Edited accommodation allocation for "+admin.GroupSummary(g), nil)
		} else {
			admin.Audit(rec, r, adminlog.ActionAllocate, &gid,
				"Allocated accommodation for "+admin.GroupSummary(g), nil)
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func postInvoice(svc *billing.Service, rec *adminlog.Recorder, regRepo *regstorage.Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body billing.VersionedBody
		_ = request.Decode(r, &body)
		groupID := chi.URLParam(r, "groupID")
		actor := admin.ActorFrom(r.Context())
		if err := svc.SendInvoice(r.Context(), groupID, actor, billing.ExpectedVersion(body.ExpectedVersion)); err != nil {
			commonerrors.WriteError(w, err)
			return
		}
		g, _ := regRepo.FindGroupByID(r.Context(), groupID)
		gid := groupID
		admin.Audit(rec, r, adminlog.ActionInvoiceSent, &gid,
			"Sent balance invoice to "+admin.GroupSummary(g), nil)
		w.WriteHeader(http.StatusNoContent)
	}
}

func postInvoiceBulk(svc *billing.Service, rec *adminlog.Recorder, regRepo *regstorage.Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body billing.BulkInvoiceRequest
		if err := request.Decode(r, &body); err != nil {
			commonerrors.WriteError(w, err)
			return
		}
		actor := admin.ActorFrom(r.Context())
		errs := svc.SendInvoicesBulk(r.Context(), actor, body.GroupIDs)
		for _, id := range body.GroupIDs {
			if _, failed := errs[id]; failed {
				continue
			}
			g, _ := regRepo.FindGroupByID(r.Context(), id)
			gid := id
			admin.Audit(rec, r, adminlog.ActionInvoiceSent, &gid,
				"Sent balance invoice to "+admin.GroupSummary(g), nil)
		}
		if len(errs) > 0 {
			render.Json(w, http.StatusMultiStatus, map[string]any{"errors": errs})
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func postCoachInvoice(svc *billing.Service, rec *adminlog.Recorder, regRepo *regstorage.Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body billing.VersionedBody
		_ = request.Decode(r, &body)
		groupID := chi.URLParam(r, "groupID")
		actor := admin.ActorFrom(r.Context())
		if err := svc.SendCoachInvoice(r.Context(), groupID, actor, billing.ExpectedVersion(body.ExpectedVersion)); err != nil {
			commonerrors.WriteError(w, err)
			return
		}
		g, _ := regRepo.FindGroupByID(r.Context(), groupID)
		gid := groupID
		admin.Audit(rec, r, adminlog.ActionCoachInvoiceSent, &gid,
			"Sent coach invoice to "+admin.GroupSummary(g), nil)
		w.WriteHeader(http.StatusNoContent)
	}
}

func postCoachInvoiceBulk(svc *billing.Service, rec *adminlog.Recorder, regRepo *regstorage.Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body billing.BulkInvoiceRequest
		if err := request.Decode(r, &body); err != nil {
			commonerrors.WriteError(w, err)
			return
		}
		actor := admin.ActorFrom(r.Context())
		errs := svc.SendCoachInvoicesBulk(r.Context(), actor, body.GroupIDs)
		for _, id := range body.GroupIDs {
			if _, failed := errs[id]; failed {
				continue
			}
			g, _ := regRepo.FindGroupByID(r.Context(), id)
			gid := id
			admin.Audit(rec, r, adminlog.ActionCoachInvoiceSent, &gid,
				"Sent coach invoice to "+admin.GroupSummary(g), nil)
		}
		if len(errs) > 0 {
			render.Json(w, http.StatusMultiStatus, map[string]any{"errors": errs})
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func postUnallocate(svc *billing.Service, rec *adminlog.Recorder, regRepo *regstorage.Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body billing.VersionedBody
		_ = request.Decode(r, &body)
		groupID := chi.URLParam(r, "groupID")
		actor := admin.ActorFrom(r.Context())
		if err := svc.Unallocate(r.Context(), groupID, actor, billing.ExpectedVersion(body.ExpectedVersion)); err != nil {
			commonerrors.WriteError(w, err)
			return
		}
		g, _ := regRepo.FindGroupByID(r.Context(), groupID)
		gid := groupID
		admin.Audit(rec, r, adminlog.ActionUnallocate, &gid,
			"Reset allocation for "+admin.GroupSummary(g), nil)
		w.WriteHeader(http.StatusNoContent)
	}
}

func postRelease(svc *billing.Service, rec *adminlog.Recorder, regRepo *regstorage.Repository, cancelled bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body billing.VersionedBody
		_ = request.Decode(r, &body)
		groupID := chi.URLParam(r, "groupID")
		actor := admin.ActorFrom(r.Context())
		reason, verb, action := "released by White Team", "Released", adminlog.ActionRelease
		if cancelled {
			reason, verb, action = "cancelled by White Team", "Cancelled", adminlog.ActionCancel
		}
		if err := svc.VoidAndRelease(r.Context(), groupID, reason, actor,
			billing.ExpectedVersion(body.ExpectedVersion), cancelled); err != nil {
			commonerrors.WriteError(w, err)
			return
		}
		g, _ := regRepo.FindGroupByID(r.Context(), groupID)
		gid := groupID
		admin.Audit(rec, r, action, &gid, verb+" allocation for "+admin.GroupSummary(g), nil)
		w.WriteHeader(http.StatusNoContent)
	}
}

func deleteCamper(svc *billing.Service, rec *adminlog.Recorder, regRepo *regstorage.Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body billing.VersionedBody
		_ = request.Decode(r, &body)
		groupID := chi.URLParam(r, "groupID")
		camperID := chi.URLParam(r, "camperID")
		actor := admin.ActorFrom(r.Context())
		sum, err := svc.RemoveCamper(r.Context(), groupID, camperID, actor, billing.ExpectedVersion(body.ExpectedVersion))
		if err != nil {
			commonerrors.WriteError(w, err)
			return
		}
		g, _ := regRepo.FindGroupByID(r.Context(), groupID)
		gid := groupID
		summary := "Removed " + sum.CamperName + " from " + admin.GroupSummary(g)
		if sum.InvoiceVoided {
			summary += "; voided open invoice"
		}
		admin.Audit(rec, r, adminlog.ActionCamperRemoved, &gid, summary, sum)
		w.WriteHeader(http.StatusNoContent)
	}
}

func postConvertDayVisitor(svc *billing.Service, rec *adminlog.Recorder, regRepo *regstorage.Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body billing.ConvertToDayVisitorRequest
		if err := request.Decode(r, &body); err != nil {
			commonerrors.WriteError(w, err)
			return
		}
		groupID := chi.URLParam(r, "groupID")
		camperID := chi.URLParam(r, "camperID")
		actor := admin.ActorFrom(r.Context())
		sum, err := svc.ConvertCamperToDayVisitor(
			r.Context(), groupID, camperID, actor, billing.ExpectedVersion(body.ExpectedVersion), body)
		if err != nil {
			commonerrors.WriteError(w, err)
			return
		}
		gid := groupID
		summary := "Converted " + sum.CamperName + " to day visitor"
		if sum.InvoiceVoided {
			summary += "; voided open invoice"
		}
		admin.Audit(rec, r, adminlog.ActionCamperConverted, &gid, summary, sum)
		w.WriteHeader(http.StatusNoContent)
	}
}

func deleteRegistration(svc *billing.Service, rec *adminlog.Recorder) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body billing.VersionedBody
		_ = request.Decode(r, &body)
		groupID := chi.URLParam(r, "groupID")
		actor := admin.ActorFrom(r.Context())
		sum, err := svc.DeleteRegistration(r.Context(), groupID, actor, billing.ExpectedVersion(body.ExpectedVersion))
		if err != nil {
			commonerrors.WriteError(w, err)
			return
		}
		summary := formatDeleteAuditSummary(sum)
		admin.Audit(rec, r, adminlog.ActionRegistrationDeleted, nil, summary, sum)
		w.WriteHeader(http.StatusNoContent)
	}
}

func formatDeleteAuditSummary(sum billing.DeleteSummary) string {
	text := "Deleted registration for " + sum.ContactName + " (" + sum.ContactEmail + ")"
	if sum.AmountPence > 0 {
		text += fmt.Sprintf("; refunded £%d.%02d", sum.AmountPence/100, sum.AmountPence%100)
	}
	if sum.InvoiceVoided {
		text += "; voided open invoice"
	}
	return text
}

func postResendInvoice(svc *billing.Service, rec *adminlog.Recorder, regRepo *regstorage.Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body billing.VersionedBody
		_ = request.Decode(r, &body)
		groupID := chi.URLParam(r, "groupID")
		actor := admin.ActorFrom(r.Context())
		if err := svc.ResendInvoice(r.Context(), groupID, actor, billing.ExpectedVersion(body.ExpectedVersion)); err != nil {
			commonerrors.WriteError(w, err)
			return
		}
		g, _ := regRepo.FindGroupByID(r.Context(), groupID)
		gid := groupID
		admin.Audit(rec, r, adminlog.ActionInvoiceResent, &gid,
			"Re-sent invoice reminder to "+admin.GroupSummary(g), nil)
		w.WriteHeader(http.StatusNoContent)
	}
}

func patchInvoiceDue(svc *billing.Service, rec *adminlog.Recorder, regRepo *regstorage.Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body billing.ExtendDueRequest
		if err := request.Decode(r, &body); err != nil {
			commonerrors.WriteError(w, err)
			return
		}
		dueAt, err := time.Parse(time.RFC3339, body.DueAt)
		if err != nil {
			commonerrors.WriteError(w, commonerrors.BadRequest("due_at must be RFC3339", nil))
			return
		}
		groupID := chi.URLParam(r, "groupID")
		actor := admin.ActorFrom(r.Context())
		if err := svc.ExtendDueAt(r.Context(), groupID, actor, billing.ExpectedVersion(body.ExpectedVersion), dueAt); err != nil {
			commonerrors.WriteError(w, err)
			return
		}
		g, _ := regRepo.FindGroupByID(r.Context(), groupID)
		gid := groupID
		admin.Audit(rec, r, adminlog.ActionExtendDue, &gid,
			"Extended invoice due date for "+admin.GroupSummary(g), nil)
		w.WriteHeader(http.StatusNoContent)
	}
}

func postBillingSweep(svc *billing.Service, rec *adminlog.Recorder) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		n, err := svc.SweepOverdue(r.Context())
		if err != nil {
			commonerrors.WriteError(w, commonerrors.Internal(err.Error()))
			return
		}
		admin.Audit(rec, r, adminlog.ActionSweep, nil,
			strings.TrimSpace(admin.ActorFrom(r.Context()))+" released "+strconv.Itoa(n)+" overdue group(s)", map[string]int{"released": n})
		render.Json(w, http.StatusOK, map[string]any{"released": n})
	}
}

func listAccommodationTypes(repo accommodationLister) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		types, err := repo.ListAccommodationTypes(r.Context())
		if err != nil {
			commonerrors.WriteError(w, commonerrors.Internal(err.Error()))
			return
		}
		render.Json(w, http.StatusOK, map[string]any{"accommodations": types})
	}
}

func listAccommodationUnits(repo unitsLister) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		units, err := repo.ListAccommodationUnits(r.Context())
		if err != nil {
			commonerrors.WriteError(w, commonerrors.Internal(err.Error()))
			return
		}
		render.Json(w, http.StatusOK, map[string]any{"units": units})
	}
}

// putAccommodationAvailability toggles whether a tier is offered on the public
// registration form. Disabling greys the tile out and rejects submissions that
// pick it; admin allocation to the tier is unaffected.
func putAccommodationAvailability(regRepo *regstorage.Repository, rec *adminlog.Recorder) http.HandlerFunc {
	type body struct {
		Available *bool `json:"available"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		var b body
		if err := request.Decode(r, &b); err != nil {
			commonerrors.WriteError(w, err)
			return
		}
		if b.Available == nil {
			commonerrors.WriteError(w, commonerrors.BadRequest("missing 'available' boolean", nil))
			return
		}
		code := chi.URLParam(r, "code")
		if err := regRepo.SetAccommodationAvailableForRegistration(r.Context(), code, *b.Available); err != nil {
			commonerrors.WriteError(w, commonerrors.Internal(err.Error()))
			return
		}
		state := "hid"
		if *b.Available {
			state = "showed"
		}
		admin.Audit(rec, r, adminlog.ActionAccommodationAvailability, nil,
			admin.ActorFrom(r.Context())+" "+state+" "+code+" on the registration form",
			map[string]any{"code": code, "available_for_registration": *b.Available})
		render.Json(w, http.StatusOK, map[string]any{
			"code":                       code,
			"available_for_registration": *b.Available,
		})
	}
}

type accommodationLister interface {
	ListAccommodationTypes(context.Context) ([]domain.AccommodationType, error)
}

type unitsLister interface {
	ListAccommodationUnits(context.Context) ([]domain.AccommodationUnit, error)
}
