package campadmin

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"pagasacentre/backend/internal/admin"
	"pagasacentre/backend/internal/billing"
	"pagasacentre/backend/internal/billing/allocation"
	"pagasacentre/backend/internal/billing/camperedit"
	"pagasacentre/backend/internal/billing/invoicing"
	usecase "pagasacentre/backend/internal/campadmin"
	"pagasacentre/backend/internal/registration/domain"
	commonerrors "pagasacentre/backend/pkg/commonlibrary/errors"
	"pagasacentre/backend/pkg/commonlibrary/render"
	"pagasacentre/backend/pkg/commonlibrary/request"
)

func putAllocation(svc *usecase.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body allocation.AllocateRequest
		if err := request.Decode(r, &body); err != nil {
			commonerrors.WriteError(w, err)
			return
		}
		groupID := chi.URLParam(r, "groupID")
		actor := admin.ActorFrom(r.Context())
		if err := svc.Allocate(r.Context(), groupID, actor, billing.ExpectedVersion(body.ExpectedVersion), body); err != nil {
			commonerrors.WriteError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func postInvoice(svc *usecase.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body billing.VersionedBody
		_ = request.Decode(r, &body)
		groupID := chi.URLParam(r, "groupID")
		actor := admin.ActorFrom(r.Context())
		if err := svc.SendInvoice(r.Context(), groupID, actor, billing.ExpectedVersion(body.ExpectedVersion)); err != nil {
			commonerrors.WriteError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func postInvoiceBulk(svc *usecase.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body billing.BulkInvoiceRequest
		if err := request.Decode(r, &body); err != nil {
			commonerrors.WriteError(w, err)
			return
		}
		actor := admin.ActorFrom(r.Context())
		errs := svc.SendInvoicesBulk(r.Context(), actor, body.GroupIDs)
		if len(errs) > 0 {
			render.Json(w, http.StatusMultiStatus, map[string]any{"errors": publicErrors(r, errs)})
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func postCoachInvoice(svc *usecase.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body billing.VersionedBody
		_ = request.Decode(r, &body)
		groupID := chi.URLParam(r, "groupID")
		actor := admin.ActorFrom(r.Context())
		if err := svc.SendCoachInvoice(r.Context(), groupID, actor, billing.ExpectedVersion(body.ExpectedVersion)); err != nil {
			commonerrors.WriteError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func postCoachInvoiceBulk(svc *usecase.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body billing.BulkInvoiceRequest
		if err := request.Decode(r, &body); err != nil {
			commonerrors.WriteError(w, err)
			return
		}
		actor := admin.ActorFrom(r.Context())
		errs := svc.SendCoachInvoicesBulk(r.Context(), actor, body.GroupIDs)
		if len(errs) > 0 {
			render.Json(w, http.StatusMultiStatus, map[string]any{"errors": publicErrors(r, errs)})
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func postWaiveCoachFee(svc *usecase.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body billing.VersionedBody
		_ = request.Decode(r, &body)
		groupID := chi.URLParam(r, "groupID")
		actor := admin.ActorFrom(r.Context())
		if err := svc.WaiveCoachFee(r.Context(), groupID, actor, billing.ExpectedVersion(body.ExpectedVersion)); err != nil {
			commonerrors.WriteError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func postUnwaiveCoachFee(svc *usecase.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body billing.VersionedBody
		_ = request.Decode(r, &body)
		groupID := chi.URLParam(r, "groupID")
		actor := admin.ActorFrom(r.Context())
		if err := svc.UnwaiveCoachFee(r.Context(), groupID, actor, billing.ExpectedVersion(body.ExpectedVersion)); err != nil {
			commonerrors.WriteError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func postUnallocate(svc *usecase.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body billing.VersionedBody
		_ = request.Decode(r, &body)
		groupID := chi.URLParam(r, "groupID")
		actor := admin.ActorFrom(r.Context())
		if err := svc.Unallocate(r.Context(), groupID, actor, billing.ExpectedVersion(body.ExpectedVersion)); err != nil {
			commonerrors.WriteError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func postRelease(svc *usecase.Service, cancelled bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body billing.VersionedBody
		_ = request.Decode(r, &body)
		groupID := chi.URLParam(r, "groupID")
		actor := admin.ActorFrom(r.Context())
		if err := svc.Release(r.Context(), groupID, actor, billing.ExpectedVersion(body.ExpectedVersion), cancelled); err != nil {
			commonerrors.WriteError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func postMarkPaid(svc *usecase.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body billing.VersionedBody
		_ = request.Decode(r, &body)
		groupID := chi.URLParam(r, "groupID")
		actor := admin.ActorFrom(r.Context())
		if err := svc.MarkBalancePaidManually(r.Context(), groupID, actor, billing.ExpectedVersion(body.ExpectedVersion)); err != nil {
			commonerrors.WriteError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func deleteCamper(svc *usecase.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body billing.VersionedBody
		_ = request.Decode(r, &body)
		groupID := chi.URLParam(r, "groupID")
		camperID := chi.URLParam(r, "camperID")
		actor := admin.ActorFrom(r.Context())
		if _, err := svc.RemoveCamper(r.Context(), groupID, camperID, actor, billing.ExpectedVersion(body.ExpectedVersion)); err != nil {
			commonerrors.WriteError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func postConvertDayVisitor(svc *usecase.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body camperedit.ConvertToDayVisitorRequest
		if err := request.Decode(r, &body); err != nil {
			commonerrors.WriteError(w, err)
			return
		}
		groupID := chi.URLParam(r, "groupID")
		camperID := chi.URLParam(r, "camperID")
		actor := admin.ActorFrom(r.Context())
		if _, err := svc.ConvertCamperToDayVisitor(
			r.Context(), groupID, camperID, actor, billing.ExpectedVersion(body.ExpectedVersion), body); err != nil {
			commonerrors.WriteError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func patchDayPassCamper(svc *usecase.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body camperedit.UpdateDayPassCamperRequest
		if err := request.Decode(r, &body); err != nil {
			commonerrors.WriteError(w, err)
			return
		}
		groupID := chi.URLParam(r, "groupID")
		camperID := chi.URLParam(r, "camperID")
		actor := admin.ActorFrom(r.Context())
		if _, err := svc.UpdateDayPassCamper(
			r.Context(), groupID, camperID, actor, billing.ExpectedVersion(body.ExpectedVersion), body); err != nil {
			commonerrors.WriteError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func patchCamperCoach(svc *usecase.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body camperedit.UpdateCamperCoachRequest
		if err := request.Decode(r, &body); err != nil {
			commonerrors.WriteError(w, err)
			return
		}
		groupID := chi.URLParam(r, "groupID")
		camperID := chi.URLParam(r, "camperID")
		actor := admin.ActorFrom(r.Context())
		if _, err := svc.UpdateCamperCoach(
			r.Context(), groupID, camperID, actor, billing.ExpectedVersion(body.ExpectedVersion), body); err != nil {
			commonerrors.WriteError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func patchCamper(svc *usecase.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body camperedit.EditCamperRequest
		if err := request.Decode(r, &body); err != nil {
			commonerrors.WriteError(w, err)
			return
		}
		groupID := chi.URLParam(r, "groupID")
		camperID := chi.URLParam(r, "camperID")
		actor := admin.ActorFrom(r.Context())
		sum, err := svc.EditCamper(
			r.Context(), groupID, camperID, actor, billing.ExpectedVersion(body.ExpectedVersion), body)
		if err != nil {
			commonerrors.WriteError(w, err)
			return
		}
		render.Json(w, http.StatusOK, sum)
	}
}

func postAddCamper(svc *usecase.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body camperedit.AddCamperRequest
		if err := request.Decode(r, &body); err != nil {
			commonerrors.WriteError(w, err)
			return
		}
		groupID := chi.URLParam(r, "groupID")
		actor := admin.ActorFrom(r.Context())
		sum, err := svc.AddCamper(r.Context(), groupID, actor, billing.ExpectedVersion(body.ExpectedVersion), body)
		if err != nil {
			commonerrors.WriteError(w, err)
			return
		}
		render.Json(w, http.StatusCreated, sum)
	}
}

func postMakeMainContact(svc *usecase.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body billing.VersionedBody
		_ = request.Decode(r, &body)
		groupID := chi.URLParam(r, "groupID")
		camperID := chi.URLParam(r, "camperID")
		actor := admin.ActorFrom(r.Context())
		if err := svc.MakeMainContact(r.Context(), groupID, camperID, actor, billing.ExpectedVersion(body.ExpectedVersion)); err != nil {
			commonerrors.WriteError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func postWaiveCamperDeposit(svc *usecase.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body billing.VersionedBody
		_ = request.Decode(r, &body)
		groupID := chi.URLParam(r, "groupID")
		camperID := chi.URLParam(r, "camperID")
		actor := admin.ActorFrom(r.Context())
		if err := svc.WaiveCamperDeposit(r.Context(), groupID, camperID, actor, billing.ExpectedVersion(body.ExpectedVersion)); err != nil {
			commonerrors.WriteError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func deleteRegistration(svc *usecase.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body billing.VersionedBody
		_ = request.Decode(r, &body)
		groupID := chi.URLParam(r, "groupID")
		actor := admin.ActorFrom(r.Context())
		if _, err := svc.DeleteRegistration(r.Context(), groupID, actor, billing.ExpectedVersion(body.ExpectedVersion)); err != nil {
			commonerrors.WriteError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func postResendInvoice(svc *usecase.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body billing.VersionedBody
		_ = request.Decode(r, &body)
		groupID := chi.URLParam(r, "groupID")
		actor := admin.ActorFrom(r.Context())
		if err := svc.ResendInvoice(r.Context(), groupID, actor, billing.ExpectedVersion(body.ExpectedVersion)); err != nil {
			commonerrors.WriteError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func postResyncAllSheets(svc *usecase.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor := admin.ActorFrom(r.Context())
		synced, errs := svc.ResyncAllSheets(r.Context(), actor)
		if len(errs) > 0 {
			render.Json(w, http.StatusMultiStatus, map[string]any{
				"synced": synced,
				"errors": publicErrors(r, errs),
			})
			return
		}
		render.Json(w, http.StatusOK, map[string]any{"synced": synced})
	}
}

func patchInvoiceDue(svc *usecase.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body invoicing.ExtendDueRequest
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
		w.WriteHeader(http.StatusNoContent)
	}
}

func postBillingSweep(svc *usecase.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		n, err := svc.SweepOverdue(r.Context(), admin.ActorFrom(r.Context()))
		if err != nil {
			commonerrors.WriteError(w, err)
			return
		}
		render.Json(w, http.StatusOK, map[string]any{"released": n})
	}
}

func listAccommodationTypes(repo accommodationLister) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		types, err := repo.ListAccommodationTypes(r.Context())
		if err != nil {
			commonerrors.WriteError(w, err)
			return
		}
		render.Json(w, http.StatusOK, map[string]any{"accommodations": types})
	}
}

func listAccommodationUnits(repo unitsLister) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		units, err := repo.ListAccommodationUnits(r.Context())
		if err != nil {
			commonerrors.WriteError(w, err)
			return
		}
		render.Json(w, http.StatusOK, map[string]any{"units": units})
	}
}

type availabilitySetter interface {
	SetAccommodationAvailability(ctx context.Context, actor, code string, available bool) error
}

func putAccommodationAvailability(setter availabilitySetter) http.HandlerFunc {
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
		if err := setter.SetAccommodationAvailability(r.Context(), admin.ActorFrom(r.Context()), code, *b.Available); err != nil {
			commonerrors.WriteError(w, err)
			return
		}
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
