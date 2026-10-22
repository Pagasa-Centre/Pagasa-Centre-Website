package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"pagasacentre/backend/pkg/commonlibrary/apperr"
)

func testAuthService(t *testing.T, now time.Time) *Service {
	t.Helper()
	return NewService(Config{
		Password:         "secret",
		SessionSecret:    []byte("test-session-secret"),
		FreeCodePassword: "free-code",
	}, nil, func() time.Time { return now })
}

func TestLogin_success(t *testing.T) {
	s := testAuthService(t, time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC))
	sess, err := s.Login(context.Background(), LoginInput{
		Password: "secret", FirstName: "Pat", LastName: "Lee", ClientKey: "client-1",
	})
	if err != nil || sess.Token == "" || sess.Name != "Pat Lee" {
		t.Fatalf("Login: %+v err=%v", sess, err)
	}
	if _, ok := s.VerifyToken(sess.Token); !ok {
		t.Fatal("token should verify immediately after login")
	}
}

func TestLogin_wrongPassword(t *testing.T) {
	s := testAuthService(t, time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC))
	_, err := s.Login(context.Background(), LoginInput{
		Password: "wrong", FirstName: "Pat", LastName: "Lee", ClientKey: "client-1",
	})
	assertKindMsg(t, err, apperr.KindUnauthorized, "invalid credentials")
}

func TestLogin_missingNames(t *testing.T) {
	s := testAuthService(t, time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC))
	_, err := s.Login(context.Background(), LoginInput{
		Password: "secret", FirstName: " ", LastName: "Lee", ClientKey: "client-1",
	})
	assertKindMsg(t, err, apperr.KindInvalid, "first and last name are required")
}

func TestLogin_rateLimitedOn11thAttempt(t *testing.T) {
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	s := testAuthService(t, now)
	key := "same-client"
	in := LoginInput{Password: "wrong", FirstName: "A", LastName: "B", ClientKey: key}
	for i := 0; i < 10; i++ {
		if _, err := s.Login(context.Background(), in); err == nil {
			t.Fatalf("attempt %d: expected error", i+1)
		}
	}
	_, err := s.Login(context.Background(), in)
	assertKindMsg(t, err, apperr.KindRateLimited, "too many attempts")
}

func TestLogin_emptyPasswordRejectedWhenUnconfigured(t *testing.T) {
	s := NewService(Config{}, nil)
	_, err := s.Login(context.Background(), LoginInput{
		Password: "", FirstName: "A", LastName: "B", ClientKey: "1",
	})
	assertKindMsg(t, err, apperr.KindUnauthorized, "invalid credentials")
}

func TestLogin_emptySessionSecretRejected(t *testing.T) {
	s := NewService(Config{Password: "secret"}, nil)
	_, err := s.Login(context.Background(), LoginInput{
		Password: "secret", FirstName: "A", LastName: "B", ClientKey: "1",
	})
	assertKindMsg(t, err, apperr.KindUnauthorized, "invalid credentials")
}

func TestVerifyToken_expired(t *testing.T) {
	issued := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	s := testAuthService(t, issued)
	sess, err := s.Login(context.Background(), LoginInput{
		Password: "secret", FirstName: "A", LastName: "B", ClientKey: "1",
	})
	if err != nil {
		t.Fatal(err)
	}
	later := issued.Add(SessionDuration + time.Second)
	sLater := NewService(Config{
		Password: "secret", SessionSecret: []byte("test-session-secret"),
	}, nil, func() time.Time { return later })
	if _, ok := sLater.VerifyToken(sess.Token); ok {
		t.Fatal("expired token should not verify")
	}
}

func TestVerifyToken_emptySecret(t *testing.T) {
	if _, ok := NewService(Config{}, nil).VerifyToken("anything"); ok {
		t.Fatal("VerifyToken with empty secret should fail")
	}
}

func TestFreeCodePasswordMatches_unconfigured(t *testing.T) {
	s := NewService(Config{}, nil)
	if s.FreeCodePasswordMatches("") || s.FreeCodePasswordMatches("x") {
		t.Fatal("expected false when FreeCodePassword unset")
	}
}

func TestFreeCodePasswordMatches_configured(t *testing.T) {
	s := testAuthService(t, time.Now())
	if !s.FreeCodePasswordMatches("free-code") {
		t.Fatal("expected match")
	}
	if s.FreeCodePasswordMatches("other") {
		t.Fatal("expected no match")
	}
}

func assertKindMsg(t *testing.T, err error, kind apperr.Kind, msg string) {
	t.Helper()
	var de *apperr.Error
	if !errors.As(err, &de) || de.Kind != kind {
		t.Fatalf("want kind %v, got %v", kind, err)
	}
	if de.Message != msg {
		t.Fatalf("message = %q, want %q", de.Message, msg)
	}
}
