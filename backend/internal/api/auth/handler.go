package auth

import (
	"net/http"
	"strings"

	authdto "pagasacentre/backend/internal/api/auth/dto"
	authsvc "pagasacentre/backend/internal/auth"
	"pagasacentre/backend/pkg/commonlibrary/apperr"
	commonerrors "pagasacentre/backend/pkg/commonlibrary/errors"
	"pagasacentre/backend/pkg/commonlibrary/render"
	"pagasacentre/backend/pkg/commonlibrary/request"
)

type tokenVerifier interface {
	VerifyToken(token string) (actor string, ok bool)
}

type Handler struct {
	svc    *authsvc.Service
	secure bool
}

func NewHandler(svc *authsvc.Service, secureCookie bool) *Handler {
	return &Handler{svc: svc, secure: secureCookie}
}

func (h *Handler) Login() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req authdto.LoginRequest
		if err := request.Decode(r, &req); err != nil {
			commonerrors.WriteError(w, err)
			return
		}
		sess, err := h.svc.Login(r.Context(), authsvc.LoginInput{
			Password:  req.Password,
			FirstName: req.FirstName,
			LastName:  req.LastName,
			ClientKey: ClientKey(r),
		})
		if err != nil {
			commonerrors.WriteError(w, err)
			return
		}
		SetSessionCookie(w, h.secure, sess.Token)
		render.Json(w, http.StatusOK, authdto.LoginFromSession(sess.Name, sess.Token))
	}
}

func (h *Handler) Logout() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ClearSessionCookie(w, h.secure)
		w.WriteHeader(http.StatusNoContent)
	}
}

func (h *Handler) Session() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := actorFromRequest(r, h.svc)
		if !ok {
			commonerrors.WriteError(w, apperr.Unauthorized("unauthorized"))
			return
		}
		render.Json(w, http.StatusOK, authdto.SessionFromName(actor))
	}
}

func actorFromRequest(r *http.Request, v tokenVerifier) (string, bool) {
	if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
		token := strings.TrimSpace(strings.TrimPrefix(h, "Bearer "))
		return v.VerifyToken(token)
	}
	c, err := r.Cookie(authsvc.SessionCookieName)
	if err != nil || c.Value == "" {
		return "", false
	}
	return v.VerifyToken(c.Value)
}
