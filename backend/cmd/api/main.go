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

	"pagasacentre/backend/internal/accommodation"
	"pagasacentre/backend/internal/admin"
	"pagasacentre/backend/internal/camp"
	"pagasacentre/backend/internal/config"
	"pagasacentre/backend/internal/consent"
	"pagasacentre/backend/internal/db"
	"pagasacentre/backend/internal/httpx"
	"pagasacentre/backend/internal/payment"
	"pagasacentre/backend/internal/registration"
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

	// Stripe
	stripeCli := payment.NewStripeClient(
		cfg.StripeSecretKey, cfg.StripeWebhookSecret,
		cfg.StripeSuccessURL, cfg.StripeCancelURL,
	)

	// Registration service depends on small interfaces; adapt camp.Repository
	// so it satisfies registration.PriceLookup with the local PriceRow type.
	regSvc := registration.NewService(
		regRepo,
		accSvc,
		campPriceAdapter{repo: campRepo},
		stripeCli,
		campRepo,
		cfg.PublicBaseURL,
	)

	// Payment service handles webhook events.
	paySvc := payment.NewService(pool, regRepo, payment.NewAccommodationLocker(accRepo), stripeCli)

	r := chi.NewRouter()
	httpx.UseDefaults(r, cfg.AllowedOrigin)

	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	r.Route("/api", func(r chi.Router) {
		camp.Mount(r, campRepo)
		accommodation.Mount(r, accSvc)
		registration.Mount(r, regSvc)
		payment.Mount(r, paySvc, stripeCli)
		consent.Mount(r)
	})

	r.Route("/admin", func(r chi.Router) {
		// TODO(auth): protect with admin auth before deploying publicly.
		admin.Mount(r, regRepo, accRepo, campRepo)
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
