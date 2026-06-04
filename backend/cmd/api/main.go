package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/robfig/cron/v3"

	"pagasacentre/backend/internal/accommodation"
	"pagasacentre/backend/internal/admin"
	"pagasacentre/backend/internal/billing"
	"pagasacentre/backend/internal/camp"
	"pagasacentre/backend/internal/config"
	"pagasacentre/backend/internal/consent"
	"pagasacentre/backend/internal/db"
	"pagasacentre/backend/internal/email"
	"pagasacentre/backend/internal/httpx"
	"pagasacentre/backend/internal/payment"
	"pagasacentre/backend/internal/registration"
	"pagasacentre/backend/internal/sheets"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	if err := db.RunMigrations(cfg.DatabaseURL, "file://migrations"); err != nil {
		log.Fatalf("migrations: %v", err)
	}

	ctx := context.Background()
	pool, err := db.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	defer pool.Close()

	// Repositories
	accRepo := accommodation.NewRepository(pool)
	accSvc := accommodation.NewService(accRepo)
	campRepo := camp.NewRepository(pool)
	regRepo := registration.NewRepository(pool)

	applyStripePriceOverrides(ctx, regRepo, cfg)

	// Stripe
	stripeCli := payment.NewStripeClient(
		cfg.StripeSecretKey, cfg.StripeWebhookSecret,
		cfg.StripeSuccessURL, cfg.StripeCancelURL,
	)

	// Email backend selection. Priority:
	//   1. Resend (HTTPS, works on hosts that block SMTP)
	//   2. SMTP (works locally and on hosts that allow 465/587 out)
	//   3. NoopMailer (template still renders, just logs)
	var mailer email.Mailer
	switch {
	case cfg.ResendAPIKey != "" && cfg.EmailFrom != "":
		mailer = email.NewResendMailer(cfg.ResendAPIKey, cfg.EmailFrom)
		log.Printf("email: Resend enabled (from=%s)", cfg.EmailFrom)
	case cfg.SMTPHost != "":
		mailer = email.NewSMTPMailer(cfg.SMTPHost, cfg.SMTPPort, cfg.SMTPUsername, cfg.SMTPPassword, cfg.SMTPFrom)
		log.Printf("email: SMTP enabled (host=%s from=%s)", cfg.SMTPHost, cfg.SMTPFrom)
	default:
		mailer = email.NewNoopMailer()
		log.Println("email: no backend configured, using NoopMailer (no real email sent)")
	}

	// Google Sheets live sync.
	var sheetSync sheets.Sync
	if cfg.GoogleServiceAccountJSON != "" && cfg.GoogleSheetsSpreadsheetID != "" {
		gs, err := sheets.NewGoogleSync(ctx, sheets.GoogleSyncConfig{
			ServiceAccountJSON: cfg.GoogleServiceAccountJSON,
			SpreadsheetID:      cfg.GoogleSheetsSpreadsheetID,
			PendingTab:         cfg.GoogleSheetsPendingTab,
			PaidTab:            cfg.GoogleSheetsPaidTab,
		})
		if err != nil {
			log.Printf("sheets: init failed, falling back to NoopSync: %v", err)
			sheetSync = sheets.NewNoopSync()
		} else {
			sheetSync = gs
			log.Printf("sheets: GoogleSync enabled (spreadsheet=%s pending=%q paid=%q)",
				cfg.GoogleSheetsSpreadsheetID, cfg.GoogleSheetsPendingTab, cfg.GoogleSheetsPaidTab)
		}
	} else {
		sheetSync = sheets.NewNoopSync()
		log.Println("sheets: GOOGLE_SERVICE_ACCOUNT_JSON or GOOGLE_SHEETS_SPREADSHEET_ID unset, using NoopSync")
	}

	regSvc := registration.NewService(
		regRepo,
		campPriceAdapter{repo: campRepo},
		stripeCli,
		campRepo,
		mailer,
		sheetSync,
		cfg.PublicBaseURL,
	)

	paySvc := payment.NewService(pool, regRepo, mailer, sheetSync, cfg.PublicBaseURL)

	billCfg := billing.Config{
		StripePriceChildUnder3: cfg.StripePriceChildUnder3,
		InvoiceDueDays:         cfg.InvoiceDueDays,
		WhiteTeamEmail:         cfg.WhiteTeamEmail,
	}
	billSvc := billing.NewService(regRepo, billing.NewStripeBilling(), mailer, billCfg)

	adminAuth := admin.AuthConfig{
		Password:      cfg.AdminPassword,
		SessionSecret: []byte(cfg.AdminSessionSecret),
		SecureCookie:  cfg.AdminSecureCookie,
	}
	if cfg.AdminPassword == "" || cfg.AdminSessionSecret == "" {
		log.Println("admin: WARNING — ADMIN_PASSWORD or ADMIN_SESSION_SECRET unset; login will not work")
	}

	// Daily sweep: release allocations unpaid past invoice_due_at.
	cronScheduler := cron.New()
	_, err = cronScheduler.AddFunc("0 3 * * *", func() {
		n, sweepErr := billSvc.SweepOverdue(context.Background())
		if sweepErr != nil {
			log.Printf("billing sweep: %v", sweepErr)
			return
		}
		if n > 0 {
			log.Printf("billing sweep: released %d overdue group(s)", n)
		}
	})
	if err != nil {
		log.Fatalf("cron: %v", err)
	}
	cronScheduler.Start()
	defer cronScheduler.Stop()

	r := chi.NewRouter()
	httpx.UseDefaults(r, cfg.AllowedOrigin)

	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	r.Route("/api", func(r chi.Router) {
		camp.Mount(r, campRepo)
		accommodation.Mount(r, accSvc)
		registration.Mount(r, regSvc)
		payment.Mount(r, paySvc, billSvc, stripeCli)
		consent.Mount(r)
	})

	r.Route("/admin", func(r chi.Router) {
		admin.Mount(r, adminAuth, regRepo, campRepo, billSvc)
	})

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("Pag-Asa Centre API listening on %s", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	log.Println("shutting down...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
}

func applyStripePriceOverrides(ctx context.Context, repo *registration.Repository, cfg config.Config) {
	pairs := map[string]string{
		"lodge":          cfg.StripePriceLodge,
		"cabin":          cfg.StripePriceCabin,
		"static_caravan": cfg.StripePriceStaticCaravan,
		"pod":            cfg.StripePricePod,
		"tent":           cfg.StripePriceTent,
		"child":          cfg.StripePriceChild312,
	}
	for code, priceID := range pairs {
		if priceID == "" {
			continue
		}
		if err := repo.UpdateAccommodationStripePrice(ctx, code, priceID); err != nil {
			log.Printf("stripe price override %s: %v", code, err)
		}
	}
}

// campPriceAdapter wraps a *camp.Repository so it satisfies
// registration.PriceLookup without leaking the camp package into registration.
type campPriceAdapter struct{ repo *camp.Repository }

func (a campPriceAdapter) GetPrice(ctx context.Context, code string) (registration.PriceRow, error) {
	p, err := a.repo.GetPrice(ctx, code)
	if err != nil {
		return registration.PriceRow{}, err
	}
	return registration.PriceRow{AmountPence: p.AmountPence, Currency: p.Currency}, nil
}
