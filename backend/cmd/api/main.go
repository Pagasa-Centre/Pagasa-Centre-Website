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

	"github.com/robfig/cron/v3"

	accomapi "pagasacentre/backend/internal/api/accommodation"
	campapi "pagasacentre/backend/internal/api/camp"
	consentapi "pagasacentre/backend/internal/api/consent"
	paymentapi "pagasacentre/backend/internal/api/payment"
	regapi "pagasacentre/backend/internal/api/registration"
	"pagasacentre/backend/internal/accommodation"
	"pagasacentre/backend/internal/adminlog"
	"pagasacentre/backend/internal/billing"
	"pagasacentre/backend/internal/camp"
	"pagasacentre/backend/internal/config"
	"pagasacentre/backend/internal/email"
	"pagasacentre/backend/internal/http/router"
	"pagasacentre/backend/internal/middleware"
	"pagasacentre/backend/internal/payment"
	"pagasacentre/backend/internal/registration"
	regstorage "pagasacentre/backend/internal/registration/storage"
	campstorage "pagasacentre/backend/internal/camp/storage"
	accomstorage "pagasacentre/backend/internal/accommodation/storage"
	"pagasacentre/backend/internal/sheets"
	commondb "pagasacentre/backend/pkg/commonlibrary/db"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	if err := commondb.RunMigrations(cfg.DatabaseURL, "file://migrations"); err != nil {
		log.Fatalf("migrations: %v", err)
	}

	ctx := context.Background()
	pool, err := commondb.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	defer pool.Close()

	accRepo := accomstorage.NewRepository(pool)
	accSvc := accommodation.NewService(accRepo)
	campRepo := campstorage.NewRepository(pool)
	campSvc := camp.NewService(campRepo)
	regRepo := regstorage.NewRepository(pool)

	applyStripePriceOverrides(ctx, regRepo, cfg)

	stripeCli := payment.NewStripeClient(
		cfg.StripeSecretKey, cfg.StripeWebhookSecret,
		cfg.StripeSuccessURL, cfg.StripeCancelURL,
	)

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
		StripePriceDayPass:     cfg.StripePriceDayPass,
		InvoiceDueDays:         cfg.InvoiceDueDays,
		WhiteTeamEmail:         cfg.WhiteTeamEmail,
	}
	billSvc := billing.NewService(regRepo, billing.NewStripeBilling(), mailer, sheetSync, billCfg)

	adminAuth := middleware.AuthConfig{
		Password:         cfg.AdminPassword,
		SessionSecret:    []byte(cfg.AdminSessionSecret),
		SecureCookie:     cfg.AdminSecureCookie,
		FreeCodePassword: cfg.AdminFreeCodePassword,
	}
	if cfg.AdminPassword == "" || cfg.AdminSessionSecret == "" {
		log.Println("admin: WARNING — ADMIN_PASSWORD or ADMIN_SESSION_SECRET unset; login will not work")
	}

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

	commit := os.Getenv("RAILWAY_GIT_COMMIT_SHA")
	if commit == "" {
		commit = "unknown"
	}
	log.Printf("build: running commit %s", commit)

	adminHub := adminlog.NewHub()
	adminRec := adminlog.NewRecorder(pool, adminHub)

	handler := router.New(router.Config{
		AllowedOrigin:        cfg.AllowedOrigin,
		Commit:               commit,
		AdminAuth:            adminAuth,
		CampHandler:          campapi.NewHandler(campSvc),
		AccommodationHandler: accomapi.NewHandler(accSvc),
		RegistrationHandler:  regapi.NewHandler(regSvc),
		PaymentHandler:       paymentapi.NewHandler(paySvc, billSvc, stripeCli),
		ConsentHandler:       consentapi.NewHandler(),
		RegRepo:              regRepo,
		CampRepo:             campRepo,
		RegSvc:               regSvc,
		BillSvc:              billSvc,
		PaySvc:               paySvc,
		AdminRec:             adminRec,
		AdminHub:             adminHub,
	})

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           handler,
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

func applyStripePriceOverrides(ctx context.Context, repo *regstorage.Repository, cfg config.Config) {
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

type campPriceAdapter struct{ repo *campstorage.Repository }

func (a campPriceAdapter) GetPrice(ctx context.Context, code string) (registration.PriceRow, error) {
	p, err := a.repo.GetPrice(ctx, code)
	if err != nil {
		return registration.PriceRow{}, err
	}
	return registration.PriceRow{AmountPence: p.AmountPence, Currency: p.Currency}, nil
}
