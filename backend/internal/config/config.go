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
	// Email config. Two possible backends, in priority order:
	//   1. Resend HTTP API (RESEND_API_KEY + EMAIL_FROM) — required on hosts
	//      that block outbound SMTP (Railway, Fly, etc.).
	//   2. SMTP (SMTP_HOST + SMTP_* + SMTP_FROM) — works locally and on hosts
	//      that allow port 465/587 out.
	// If neither is configured, a NoopMailer is used (template still renders,
	// just logs instead of sending).
	ResendAPIKey string
	EmailFrom    string
	SMTPHost     string
	SMTPPort     string
	SMTPUsername string
	SMTPPassword string
	SMTPFrom     string

	// Google Sheets live sync. All three required to enable; if any is
	// empty the app boots with a NoopSync (no rows written).
	GoogleServiceAccountJSON string
	GoogleSheetsSpreadsheetID string
	GoogleSheetsPendingTab    string // defaults to "Pending"
	GoogleSheetsPaidTab       string // defaults to "Paid"
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
		ResendAPIKey:              os.Getenv("RESEND_API_KEY"),
		EmailFrom:                 os.Getenv("EMAIL_FROM"),
		SMTPHost:                  os.Getenv("SMTP_HOST"),
		SMTPPort:                  getEnv("SMTP_PORT", "587"),
		SMTPUsername:              os.Getenv("SMTP_USERNAME"),
		SMTPPassword:              os.Getenv("SMTP_PASSWORD"),
		SMTPFrom:                  os.Getenv("SMTP_FROM"),
		GoogleServiceAccountJSON:  os.Getenv("GOOGLE_SERVICE_ACCOUNT_JSON"),
		GoogleSheetsSpreadsheetID: os.Getenv("GOOGLE_SHEETS_SPREADSHEET_ID"),
		GoogleSheetsPendingTab:    getEnv("GOOGLE_SHEETS_PENDING_TAB", "Pending"),
		GoogleSheetsPaidTab:       getEnv("GOOGLE_SHEETS_PAID_TAB", "Paid"),
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
