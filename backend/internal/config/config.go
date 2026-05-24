package config

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	Port                string
	DatabaseURL         string
	StripeSecretKey     string
	StripeWebhookSecret string
	StripeSuccessURL    string
	StripeCancelURL     string
	AllowedOrigin       string
	PublicBaseURL       string
	// SMTP for transactional emails. All optional — if SMTPHost is empty the
	// app boots with a NoopMailer (local dev, no real email sent).
	SMTPHost     string
	SMTPPort     string
	SMTPUsername string
	SMTPPassword string
	SMTPFrom     string
}

func Load() (Config, error) {
	_ = godotenv.Load()

	cfg := Config{
		Port:                getEnv("PORT", "8080"),
		DatabaseURL:         os.Getenv("DATABASE_URL"),
		StripeSecretKey:     os.Getenv("STRIPE_SECRET_KEY"),
		StripeWebhookSecret: os.Getenv("STRIPE_WEBHOOK_SECRET"),
		StripeSuccessURL:    os.Getenv("STRIPE_SUCCESS_URL"),
		StripeCancelURL:     os.Getenv("STRIPE_CANCEL_URL"),
		AllowedOrigin:       getEnv("CORS_ALLOWED_ORIGIN", "http://localhost:3000"),
		PublicBaseURL:       getEnv("PUBLIC_BASE_URL", "http://localhost:8080"),
		SMTPHost:            os.Getenv("SMTP_HOST"),
		SMTPPort:            getEnv("SMTP_PORT", "587"),
		SMTPUsername:        os.Getenv("SMTP_USERNAME"),
		SMTPPassword:        os.Getenv("SMTP_PASSWORD"),
		SMTPFrom:            os.Getenv("SMTP_FROM"),
	}

	var missing []string
	if cfg.DatabaseURL == "" {
		missing = append(missing, "DATABASE_URL")
	}
	if cfg.StripeSecretKey == "" {
		missing = append(missing, "STRIPE_SECRET_KEY")
	}
	if cfg.StripeWebhookSecret == "" {
		missing = append(missing, "STRIPE_WEBHOOK_SECRET")
	}
	if cfg.StripeSuccessURL == "" {
		missing = append(missing, "STRIPE_SUCCESS_URL")
	}
	if cfg.StripeCancelURL == "" {
		missing = append(missing, "STRIPE_CANCEL_URL")
	}
	if len(missing) > 0 {
		return Config{}, fmt.Errorf("missing required env vars: %s", strings.Join(missing, ", "))
	}
	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// ErrMissingRequired is returned when a required env var is unset.
var ErrMissingRequired = errors.New("missing required env var")
