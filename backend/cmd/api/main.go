package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/robfig/cron/v3"

	"pagasacentre/backend/internal/accommodation"
	accomstorage "pagasacentre/backend/internal/accommodation/storage"
	"pagasacentre/backend/internal/adminlog"
	adminstorage "pagasacentre/backend/internal/adminlog/storage"
	accomapi "pagasacentre/backend/internal/api/accommodation"
	campapi "pagasacentre/backend/internal/api/camp"
	consentapi "pagasacentre/backend/internal/api/consent"
	paymentapi "pagasacentre/backend/internal/api/payment"
	regapi "pagasacentre/backend/internal/api/registration"
	sermonapi "pagasacentre/backend/internal/api/sermon"
	"pagasacentre/backend/internal/billing"
	"pagasacentre/backend/internal/billing/allocation"
	allocstorage "pagasacentre/backend/internal/billing/allocation/storage"
	"pagasacentre/backend/internal/billing/camperedit"
	camperstorage "pagasacentre/backend/internal/billing/camperedit/storage"
	"pagasacentre/backend/internal/billing/invoicing"
	invstorage "pagasacentre/backend/internal/billing/invoicing/storage"
	"pagasacentre/backend/internal/billing/reconcile"
	reconcilestorage "pagasacentre/backend/internal/billing/reconcile/storage"
	"pagasacentre/backend/internal/billing/sheetsync"
	sheetstorage "pagasacentre/backend/internal/billing/sheetsync/storage"
	"pagasacentre/backend/internal/billing/sweep"
	sweepstorage "pagasacentre/backend/internal/billing/sweep/storage"
	"pagasacentre/backend/internal/camp"
	campstorage "pagasacentre/backend/internal/camp/storage"
	"pagasacentre/backend/internal/campadmin"
	"pagasacentre/backend/internal/config"
	"pagasacentre/backend/internal/email"
	"pagasacentre/backend/internal/http/router"
	"pagasacentre/backend/internal/middleware"
	"pagasacentre/backend/internal/payment"
	"pagasacentre/backend/internal/registration"
	regstorage "pagasacentre/backend/internal/registration/storage"
	"pagasacentre/backend/internal/sermon"
	sermonstorage "pagasacentre/backend/internal/sermon/storage"
	"pagasacentre/backend/internal/sheets"
	commondb "pagasacentre/backend/pkg/commonlibrary/db"
)

// main is the composition root: the one place that knows concrete types and
// connects them. A production API's main should do these steps, in this order,
// and almost nothing else. Business rules live in services, SQL in repositories,
// HTTP in handlers.
//
//  1. Load config and fail fast if a required setting is missing.
//  2. Migrate the database before accepting traffic.
//  3. Open a connection pool, and close it when the process exits.
//  4. Construct dependencies bottom-up: repositories, then services, then handlers.
//  5. Start background jobs, and stop them on shutdown.
//  6. Build one HTTP handler from those dependencies.
//  7. Serve it on an http.Server that has timeouts, in a goroutine.
//  8. Block until SIGINT/SIGTERM, then shut down with a deadline.
func main() {
	// Non-zero exit after the defers below have closed the pool and stopped jobs.
	// os.Exit would skip those defers, so it runs from the first defer, which
	// is the last to execute.
	exitCode := 0
	defer func() {
		if exitCode != 0 {
			os.Exit(exitCode)
		}
	}()

	// Cancelled when the process is asked to shut down, so an in-flight billing
	// sweep or sermon sync stops instead of running against a closing database.
	jobCtx, stopJobs := context.WithCancel(context.Background())
	defer stopJobs()

	// 1. Config. Read the environment once, at the edge. Fail the process now
	// if something required is missing — a half-configured server that starts
	// and then errors on the first request is harder to diagnose.
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	// 2. Migrations. Apply schema changes before any query runs, so the code
	// and the database cannot disagree. Small services do this on boot. Larger
	// ones often run the same migrations as a separate deploy step so several
	// replicas don't migrate at once.
	if err := commondb.RunMigrations(cfg.DatabaseURL, "file://migrations"); err != nil {
		log.Fatalf("migrations: %v", err)
	}

	// 3. Database pool. One long-lived pool shared by every repository. defer
	// Close runs when main returns, including after a graceful shutdown, so
	// connections are not left open on the database.
	ctx := context.Background()
	pool, err := commondb.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	defer pool.Close()

	// 4. Dependencies, bottom-up. Repositories need the pool. Services need
	// repositories and external clients. Handlers need services. Nothing below
	// main imports a concrete constructor from another layer, so tests can
	// substitute fakes and the dependency rule stays intact.
	accRepo := accomstorage.NewRepository(pool)
	accSvc := accommodation.NewService(accRepo)
	campRepo := campstorage.NewRepository(pool)
	campSvc := camp.NewService(campRepo)
	regRepo := regstorage.NewRepository(pool)
	txRunner := commondb.NewTxRunner(pool)

	applyStripePriceOverrides(ctx, regRepo, cfg)

	stripeCli := payment.NewStripeClient(
		cfg.StripeSecretKey, cfg.StripeWebhookSecret,
		cfg.StripeSuccessURL, cfg.StripeCancelURL,
	)

	// Optional integrations. If a required client (the database, Stripe) cannot
	// start, the process should exit. If an optional one cannot, fall back to a
	// no-op and log it, so one missing integration does not take the API down.
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
		txRunner,
		campPriceAdapter{repo: campRepo},
		stripeCli,
		payment.NewPriceCatalog(),
		campRepo,
		mailer,
		sheetSync,
		cfg.PublicBaseURL,
		registration.Config{
			StripePriceChildUnder3: cfg.StripePriceChildUnder3,
			StripePriceDayPass:     cfg.StripePriceDayPass,
			StripePriceCoach:       cfg.StripePriceCoach,
		},
	)

	paySvc := payment.NewService(txRunner, regRepo, mailer, sheetSync, cfg.PublicBaseURL)

	depositPence := 5000
	if p, err := campRepo.GetPrice(ctx, "deposit"); err == nil && p.AmountPence > 0 {
		depositPence = p.AmountPence
	}

	billCfg := billing.Config{
		StripePriceChildUnder3: cfg.StripePriceChildUnder3,
		StripePriceDayPass:     cfg.StripePriceDayPass,
		StripePriceCoach:       cfg.StripePriceCoach,
		DepositPricePence:      depositPence,
		InvoiceDueDays:         cfg.InvoiceDueDays,
		WhiteTeamEmail:         cfg.WhiteTeamEmail,
	}
	pricer := billing.NewPricer(regRepo, billCfg, campPriceAdapter{repo: campRepo})
	stripeBill := billing.NewStripeBilling()
	allocRepo := allocstorage.NewRepository(pool)
	invRepo := invstorage.NewRepository(pool)
	camperRepo := camperstorage.NewRepository(pool)
	reconRepo := reconcilestorage.NewRepository(pool)
	sweepRepo := sweepstorage.NewRepository(pool)
	sheetRepo := sheetstorage.NewRepository(pool)

	allocSvc := allocation.NewService(regRepo, allocRepo, stripeBill, mailer, pricer, billCfg)
	invSvc := invoicing.NewService(regRepo, invRepo, stripeBill, mailer, sheetSync, pricer, billCfg)
	camperSvc := camperedit.NewService(regRepo, camperRepo, stripeBill, mailer, sheetSync, pricer, billCfg)
	reconSvc := reconcile.NewService(regRepo, reconRepo, mailer, pricer, billCfg)
	sweepSvc := sweep.NewService(sweepRepo, allocSvc)
	sheetSvc := sheetsync.NewService(regRepo, sheetRepo, sheetSync)

	adminAuth := middleware.AuthConfig{
		Password:         cfg.AdminPassword,
		SessionSecret:    []byte(cfg.AdminSessionSecret),
		SecureCookie:     cfg.AdminSecureCookie,
		FreeCodePassword: cfg.AdminFreeCodePassword,
	}
	if cfg.AdminPassword == "" || cfg.AdminSessionSecret == "" {
		log.Println("admin: WARNING — ADMIN_PASSWORD or ADMIN_SESSION_SECRET unset; login will not work")
	}

	// 5. Background work. Jobs live in this process, so they must start after the
	// services they call exist, and stop when the process is asked to exit
	// (defer Stop). A job that fails should be logged, not fatal — the HTTP
	// server is still useful. With more than one replica, a job inside the API
	// runs once per replica; move it out if that would double-charge or double-send.
	cronScheduler := cron.New()
	_, err = cronScheduler.AddFunc("0 3 * * *", func() {
		n, sweepErr := sweepSvc.SweepOverdue(jobCtx)
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

	sermonRepo := sermonstorage.NewRepository(pool)
	var sermonFinder sermon.LiveFinder
	if cfg.YouTubeAPIKey != "" && cfg.YouTubeChannelID != "" {
		sermonFinder = sermon.NewYouTubeClient(cfg.YouTubeAPIKey, cfg.YouTubeChannelID)
	} else {
		log.Println("youtube: YOUTUBE_API_KEY or YOUTUBE_CHANNEL_ID unset; latest sermon sync disabled")
	}
	sermonSvc := sermon.NewService(sermonRepo, sermonFinder)
	var startupSync sync.WaitGroup
	if sermonFinder != nil {
		startupSync.Add(1)
		go func() {
			defer startupSync.Done()
			if err := sermonSvc.Sync(jobCtx); err != nil {
				log.Printf("sermon sync (startup): %v", err)
			}
		}()
		london, locErr := time.LoadLocation("Europe/London")
		if locErr != nil {
			log.Fatalf("sermon cron location: %v", locErr)
		}
		sermonCron := cron.New(cron.WithLocation(london))
		_, err = sermonCron.AddFunc("0 19 * * 0", func() {
			if syncErr := sermonSvc.Sync(jobCtx); syncErr != nil {
				log.Printf("sermon sync (weekly): %v", syncErr)
			}
		})
		if err != nil {
			log.Fatalf("sermon cron: %v", err)
		}
		sermonCron.Start()
		defer sermonCron.Stop()
	}

	// Record which build is running. /health reports this so you can tell a
	// deploy finished and which commit is answering requests.
	commit := os.Getenv("RAILWAY_GIT_COMMIT_SHA")
	if commit == "" {
		commit = "unknown"
	}
	log.Printf("build: running commit %s", commit)

	adminHub := adminlog.NewHub()
	adminRec := adminlog.NewRecorder(adminstorage.NewRepository(pool), adminHub)
	adminSvc := campadmin.NewService(campadmin.BillingServices{
		Allocation: allocSvc,
		Invoicing:  invSvc,
		Campers:    camperSvc,
		Sweep:      sweepSvc,
		Sheets:     sheetSvc,
	}, regSvc, campSvc, paySvc, adminRec)

	// 6. One HTTP handler. Routes are registered in the router, not here. main
	// only hands the router the dependencies it needs.
	handler := router.New(router.Config{
		AllowedOrigin:        cfg.AllowedOrigin,
		Commit:               commit,
		AdminAuth:            adminAuth,
		DB:                   pool,
		CampHandler:          campapi.NewHandler(campSvc),
		AccommodationHandler: accomapi.NewHandler(accSvc),
		RegistrationHandler:  regapi.NewHandler(regSvc),
		PaymentHandler:       paymentapi.NewHandler(paySvc, reconSvc, stripeCli),
		ConsentHandler:       consentapi.NewHandler(),
		SermonHandler:        sermonapi.NewHandler(sermonSvc),
		CampSvc:              campSvc,
		RegSvc:               regSvc,
		AdminSvc:             adminSvc,
		AdminHub:             adminHub,
	})

	// 7. http.Server, not http.ListenAndServe. The Server value is what you shut
	// down later, and it is where timeouts live. ReadHeaderTimeout stops a
	// client that connects and never sends headers from holding a goroutine
	// open (Slowloris). Read, write, and idle timeouts are the next ones to add
	// when handlers should not run forever.
	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	// Listen in a goroutine so main can wait for a shutdown signal. A listen
	// failure is sent back here instead of calling log.Fatalf in the goroutine,
	// which would skip the defers that close the pool and stop the jobs.
	// http.ErrServerClosed is the normal error from Shutdown.
	listenErr := make(chan error, 1)
	go func() {
		log.Printf("Pag-Asa Centre API listening on %s", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			listenErr <- err
		}
	}()

	// 8. Graceful shutdown. Kubernetes, Railway, and a Ctrl-C all deliver
	// SIGTERM or SIGINT. Cancel background work first, stop accepting new
	// connections, let in-flight requests finish, then return so the defers
	// above stop the jobs and close the pool. The timeout is the deadline:
	// after it, remaining requests are dropped rather than hanging the deploy.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	select {
	case sig := <-stop:
		log.Printf("shutting down: %s", sig)
	case err := <-listenErr:
		log.Printf("server: %v", err)
		exitCode = 1
	}
	stopJobs()
	startupSync.Wait()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("server shutdown: %v", err)
	}
}

func applyStripePriceOverrides(ctx context.Context, repo *regstorage.Repository, cfg config.Config) {
	pairs := map[string]string{
		"lodge":            cfg.StripePriceLodge,
		"cabin":            cfg.StripePriceCabin,
		"static_caravan":   cfg.StripePriceStaticCaravan,
		"pod":              cfg.StripePricePod,
		"tent":             cfg.StripePriceTent,
		"caravan_overflow": cfg.StripePriceTent,
		"child":            cfg.StripePriceChild312,
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
