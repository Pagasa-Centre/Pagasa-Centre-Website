package middleware

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"pagasacentre/backend/internal/admin"
	"pagasacentre/backend/internal/adminlog"
)

const (
	sessionCookieName = "pc_camp_admin_session"
	sessionDuration   = 7 * 24 * time.Hour
	maxLoginAttempts  = 10
	loginWindow       = 15 * time.Minute
)

// AuthConfig holds shared-password admin session settings.
type AuthConfig struct {
	Password          string
	SessionSecret     []byte
	SecureCookie      bool // true in production (HTTPS)
	FreeCodePassword  string
}

// FreeCodePasswordMatches reports whether pw matches the sponsored-code generation password.
func (c AuthConfig) FreeCodePasswordMatches(pw string) bool {
	if c.FreeCodePassword == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(pw), []byte(c.FreeCodePassword)) == 1
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

type sessionPayload struct {
	Exp  int64  `json:"exp"`
	Name string `json:"name"`
}

func signSession(secret []byte, expUnix int64, name string) string {
	raw, _ := json.Marshal(sessionPayload{Exp: expUnix, Name: name})
	enc := base64.RawURLEncoding.EncodeToString(raw)
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(enc))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return enc + "." + sig
}

// VerifySessionToken validates a bearer/cookie token and returns the actor name.
func VerifySessionToken(secret []byte, token string) (string, bool) {
	if len(secret) == 0 || token == "" {
		return "", false
	}
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return "", false
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return "", false
	}
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(parts[0]))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if subtle.ConstantTimeCompare([]byte(sig), []byte(parts[1])) != 1 {
		return "", false
	}
	var p sessionPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return "", false
	}
	if p.Name == "" || time.Now().Unix() > p.Exp {
		return "", false
	}
	return p.Name, true
}

func verifySession(secret []byte, token string) bool {
	_, ok := VerifySessionToken(secret, token)
	return ok
}

func actorFromRequest(r *http.Request, cfg AuthConfig) (string, bool) {
	if len(cfg.SessionSecret) == 0 {
		return "", false
	}
	if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
		token := strings.TrimSpace(strings.TrimPrefix(h, "Bearer "))
		return VerifySessionToken(cfg.SessionSecret, token)
	}
	c, err := r.Cookie(sessionCookieName)
	if err != nil || c.Value == "" {
		return "", false
	}
	return VerifySessionToken(cfg.SessionSecret, c.Value)
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

func newSessionToken(cfg AuthConfig, name string) string {
	exp := time.Now().Add(sessionDuration).Unix()
	return signSession(cfg.SessionSecret, exp, name)
}

func setSessionCookie(w http.ResponseWriter, cfg AuthConfig, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/camp-admin",
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
		Path:     "/camp-admin",
		HttpOnly: true,
		Secure:   cfg.SecureCookie,
		SameSite: sameSiteMode(cfg),
		MaxAge:   -1,
	})
}

func isAuthenticated(r *http.Request, cfg AuthConfig) bool {
	_, ok := actorFromRequest(r, cfg)
	return ok
}

// RequireAdmin wraps handlers that need an active admin session.
func RequireAdmin(cfg AuthConfig, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		actor, ok := actorFromRequest(r, cfg)
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r.WithContext(admin.WithActor(r.Context(), actor)))
	})
}

func HandleLogin(cfg AuthConfig, rec *adminlog.Recorder) http.HandlerFunc {
	return handleLogin(cfg, rec)
}

func HandleLogout(cfg AuthConfig) http.HandlerFunc {
	return handleLogout(cfg)
}

func HandleSession(cfg AuthConfig) http.HandlerFunc {
	return handleSession(cfg)
}

func handleLogin(cfg AuthConfig, rec *adminlog.Recorder) http.HandlerFunc {
	type body struct {
		Password  string `json:"password"`
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
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
		b.FirstName = strings.TrimSpace(b.FirstName)
		b.LastName = strings.TrimSpace(b.LastName)
		if b.FirstName == "" || b.LastName == "" {
			http.Error(w, "first and last name are required", http.StatusBadRequest)
			return
		}
		if subtle.ConstantTimeCompare([]byte(b.Password), []byte(cfg.Password)) != 1 {
			http.Error(w, "invalid credentials", http.StatusUnauthorized)
			return
		}
		name := fmt.Sprintf("%s %s", b.FirstName, b.LastName)
		token := newSessionToken(cfg, name)
		setSessionCookie(w, cfg, token)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"token": token, "name": name})
		if rec != nil {
			_, _ = rec.Record(r.Context(), name, adminlog.ActionLogin, nil, name+" signed in", nil)
		}
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
		actor, ok := actorFromRequest(r, cfg)
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"name": actor})
	}
}

func decodeLoginBody(r *http.Request, dst any) error {
	const maxBody = 1 << 20
	body, err := io.ReadAll(io.LimitReader(r.Body, maxBody))
	if err != nil {
		return err
	}
	return json.Unmarshal(body, dst)
}
