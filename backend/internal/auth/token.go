package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"strings"
	"time"
)

const SessionDuration = 7 * 24 * time.Hour

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

func verifySession(secret []byte, token string, now time.Time) (string, bool) {
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
	if p.Name == "" || now.Unix() > p.Exp {
		return "", false
	}
	return p.Name, true
}
