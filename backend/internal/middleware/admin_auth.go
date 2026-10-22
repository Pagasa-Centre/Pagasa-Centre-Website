package middleware

import (
	"net/http"
	"strings"

	"pagasacentre/backend/internal/auth"
	commonerrors "pagasacentre/backend/pkg/commonlibrary/errors"
)

type tokenVerifier interface {
	VerifyToken(token string) (actor string, ok bool)
}

// RequireAdmin wraps handlers that need an active admin session.
func RequireAdmin(v tokenVerifier, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		actor, ok := actorFromRequest(r, v)
		if !ok {
			commonerrors.WriteError(w, commonerrors.APIError{Code: "unauthorized", Message: "unauthorized"})
			return
		}
		next.ServeHTTP(w, r.WithContext(auth.WithActor(r.Context(), actor)))
	})
}

func actorFromRequest(r *http.Request, v tokenVerifier) (string, bool) {
	if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
		token := strings.TrimSpace(strings.TrimPrefix(h, "Bearer "))
		return v.VerifyToken(token)
	}
	c, err := r.Cookie(auth.SessionCookieName)
	if err != nil || c.Value == "" {
		return "", false
	}
	return v.VerifyToken(c.Value)
}
