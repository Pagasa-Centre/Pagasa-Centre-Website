package admin

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	sessionCookieName = "pc_admin_session"
	sessionDuration   = 7 * 24 * time.Hour
	maxLoginAttempts  = 10
	loginWindow       = 15 * time.Minute
)

// AuthConfig holds shared-password admin session settings.
type AuthConfig struct {
	Password      string
	SessionSecret []byte
	SecureCookie  bool // true in production (HTTPS)
}

type loginLimiter struct {
	mu       sync.Mutex
	attempts map[string][]time.Time
}

func (l *loginLimiter) allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	cutoff := now.Add(-loginWindow)
	var kept []time.Time
	for _, t := range l.attempts[key] {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	if len(kept) >= maxLoginAttempts {
		l.attempts[key] = kept
		return false
	}
	kept = append(kept, now)
	l.attempts[key] = kept
	return true
}

var globalLoginLimiter = loginLimiter{
	attempts: make(map[string][]time.Time),
}

func clientKey(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return strings.TrimSpace(strings.Split(xff, ",")[0])
	}
	return r.RemoteAddr
}

func signSession(secret []byte, expUnix int64) string {
	payload := fmt.Sprintf("%d", expUnix)
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(payload))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return payload + "." + sig
}

func verifySession(secret []byte, token string) bool {
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return false
	}
	expUnix, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return false
	}
	if time.Now().Unix() > expUnix {
		return false
	}
	expected := signSession(secret, expUnix)
	return subtle.ConstantTimeCompare([]byte(expected), []byte(token)) == 1
}

// sameSiteMode picks the cookie SameSite policy. In production the admin
// dashboard (frontend domain) and the API (backend domain) are different
// Railway subdomains, so the session cookie is cross-site: it must be
// SameSite=None (which requires Secure=true) or the browser silently drops
// it on cross-origin fetches and every post-login request looks logged-out.
// Locally everything is same-origin on localhost, so Lax is fine.
func sameSiteMode(cfg AuthConfig) http.SameSite {
	if cfg.SecureCookie {
		return http.SameSiteNoneMode
	}
	return http.SameSiteLaxMode
}

func setSessionCookie(w http.ResponseWriter, cfg AuthConfig) {
	exp := time.Now().Add(sessionDuration).Unix()
	token := signSession(cfg.SessionSecret, exp)
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/admin",
		HttpOnly: true,
		Secure:   cfg.SecureCookie,
		SameSite: sameSiteMode(cfg),
		MaxAge:   int(sessionDuration.Seconds()),
	})
}

func clearSessionCookie(w http.ResponseWriter, cfg AuthConfig) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/admin",
		HttpOnly: true,
		Secure:   cfg.SecureCookie,
		SameSite: sameSiteMode(cfg),
		MaxAge:   -1,
	})
}

func isAuthenticated(r *http.Request, cfg AuthConfig) bool {
	if len(cfg.SessionSecret) == 0 {
		return false
	}
	c, err := r.Cookie(sessionCookieName)
	if err != nil || c.Value == "" {
		return false
	}
	return verifySession(cfg.SessionSecret, c.Value)
}

// RequireAdmin wraps handlers that need an active admin session.
func RequireAdmin(cfg AuthConfig, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !isAuthenticated(r, cfg) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func handleLogin(cfg AuthConfig) http.HandlerFunc {
	type body struct {
		Password string `json:"password"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		key := clientKey(r)
		if !globalLoginLimiter.allow(key) {
			http.Error(w, "too many attempts", http.StatusTooManyRequests)
			return
		}
		var b body
		if err := decodeLoginBody(r, &b); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		if subtle.ConstantTimeCompare([]byte(b.Password), []byte(cfg.Password)) != 1 {
			http.Error(w, "invalid credentials", http.StatusUnauthorized)
			return
		}
		setSessionCookie(w, cfg)
		w.WriteHeader(http.StatusNoContent)
	}
}

func handleLogout(cfg AuthConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		clearSessionCookie(w, cfg)
		w.WriteHeader(http.StatusNoContent)
	}
}

func handleSession(cfg AuthConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if isAuthenticated(r, cfg) {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	}
}
