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

	// Admin dashboard (shared password + HMAC session cookie).
	AdminPassword       string
	AdminSessionSecret  string
	AdminSecureCookie   bool // ADMIN_SECURE_COOKIE=1 behind HTTPS
	AdminFreeCodePassword string
	WhiteTeamEmail      string
	StripePriceChildUnder3 string // Stripe Price for full-week under-3 balance (£0)
	StripePriceDayPass  string // Stripe Price for a day pass (per day; quantity = days)
	InvoiceDueDays      int    // defaults to 15

	// Optional overrides for accommodation_types.stripe_price_id at boot.
	StripePriceLodge          string
	StripePriceCabin          string
	StripePriceStaticCaravan  string
	StripePricePod            string
	StripePriceTent           string
	StripePriceChild312       string // 3-12 child with parent
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
		AdminPassword:             os.Getenv("ADMIN_PASSWORD"),
		AdminSessionSecret:        os.Getenv("ADMIN_SESSION_SECRET"),
		AdminSecureCookie:         os.Getenv("ADMIN_SECURE_COOKIE") == "1" || os.Getenv("ADMIN_SECURE_COOKIE") == "true",
		AdminFreeCodePassword:     os.Getenv("ADMIN_FREE_CODE_PASSWORD"),
		WhiteTeamEmail:            os.Getenv("WHITE_TEAM_EMAIL"),
		StripePriceChildUnder3:    os.Getenv("STRIPE_PRICE_CHILD_UNDER3"),
		StripePriceDayPass:        os.Getenv("STRIPE_PRICE_DAY_PASS"),
		InvoiceDueDays:            getEnvInt("INVOICE_DUE_DAYS", 15),
		StripePriceLodge:          os.Getenv("STRIPE_PRICE_LODGE"),
		StripePriceCabin:          os.Getenv("STRIPE_PRICE_CABIN"),
		StripePriceStaticCaravan:  os.Getenv("STRIPE_PRICE_STATIC_CARAVAN"),
		StripePricePod:            os.Getenv("STRIPE_PRICE_POD"),
		StripePriceTent:           os.Getenv("STRIPE_PRICE_TENT"),
		StripePriceChild312:       os.Getenv("STRIPE_PRICE_CHILD_3_12"),
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

func getEnvInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	var n int
	if _, err := fmt.Sscanf(v, "%d", &n); err != nil || n <= 0 {
		return fallback
	}
	return n
}

// ErrMissingRequired is returned when a required env var is unset.
var ErrMissingRequired = errors.New("missing required env var")
