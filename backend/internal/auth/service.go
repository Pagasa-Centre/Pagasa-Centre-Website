package auth

import (
	"context"
	"crypto/subtle"
	"fmt"
	"strings"
	"time"

	"pagasacentre/backend/pkg/commonlibrary/apperr"
)

type Config struct {
	Password         string
	SessionSecret    []byte
	FreeCodePassword string
}

type LoginInput struct {
	Password  string
	FirstName string
	LastName  string
	ClientKey string
}

type Session struct {
	Token string
	Name  string
}

type loginRecorder interface {
	RecordLogin(ctx context.Context, name string)
}

type Service struct {
	cfg     Config
	rec     loginRecorder
	limiter *loginLimiter
	now     func() time.Time
}

func NewService(cfg Config, rec loginRecorder, now ...func() time.Time) *Service {
	n := time.Now
	if len(now) > 0 && now[0] != nil {
		n = now[0]
	}
	return &Service{
		cfg:     cfg,
		rec:     rec,
		limiter: newLoginLimiter(n),
		now:     n,
	}
}

func (s *Service) Login(ctx context.Context, in LoginInput) (Session, error) {
	if !s.limiter.allow(in.ClientKey) {
		return Session{}, apperr.RateLimited("too many attempts")
	}
	first := strings.TrimSpace(in.FirstName)
	last := strings.TrimSpace(in.LastName)
	if first == "" || last == "" {
		return Session{}, apperr.Invalid("first and last name are required")
	}
	if len(s.cfg.Password) == 0 || len(s.cfg.SessionSecret) == 0 ||
		subtle.ConstantTimeCompare([]byte(in.Password), []byte(s.cfg.Password)) != 1 {
		return Session{}, apperr.Unauthorized("invalid credentials")
	}
	name := fmt.Sprintf("%s %s", first, last)
	exp := s.now().Add(SessionDuration).Unix()
	token := signSession(s.cfg.SessionSecret, exp, name)
	if s.rec != nil {
		s.rec.RecordLogin(ctx, name)
	}
	return Session{Token: token, Name: name}, nil
}

func (s *Service) VerifyToken(token string) (actor string, ok bool) {
	return verifySession(s.cfg.SessionSecret, token, s.now())
}

func (s *Service) FreeCodePasswordMatches(pw string) bool {
	if s.cfg.FreeCodePassword == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(pw), []byte(s.cfg.FreeCodePassword)) == 1
}

func (s *Service) SponsorshipCodesEnabled() bool {
	return s.cfg.FreeCodePassword != ""
}
