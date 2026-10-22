package auth

import (
	"net/http"
	"strings"

	authsvc "pagasacentre/backend/internal/auth"
)

func sameSiteMode(secure bool) http.SameSite {
	if secure {
		return http.SameSiteNoneMode
	}
	return http.SameSiteLaxMode
}

func SetSessionCookie(w http.ResponseWriter, secure bool, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     authsvc.SessionCookieName,
		Value:    token,
		Path:     "/camp-admin",
		HttpOnly: true,
		Secure:   secure,
		SameSite: sameSiteMode(secure),
		MaxAge:   int(authsvc.SessionDuration.Seconds()),
	})
}

func ClearSessionCookie(w http.ResponseWriter, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     authsvc.SessionCookieName,
		Value:    "",
		Path:     "/camp-admin",
		HttpOnly: true,
		Secure:   secure,
		SameSite: sameSiteMode(secure),
		MaxAge:   -1,
	})
}

func ClientKey(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return strings.TrimSpace(strings.Split(xff, ",")[0])
	}
	return r.RemoteAddr
}
