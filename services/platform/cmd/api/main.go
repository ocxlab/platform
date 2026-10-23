package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"ocp.local/platform/internal/infra/database"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"ocp.local/platform/cmd/api/config"
	"ocp.local/platform/cmd/api/transport/httpapi"
	"ocp.local/platform/cmd/api/transport/ucpinternal"
	"ocp.local/platform/internal/core/capability"
	"ocp.local/platform/internal/core/updates"
	"ocp.local/platform/internal/core/work"
	"ocp.local/platform/internal/discovery"
	"ocp.local/platform/internal/infra/media"
	"ocp.local/platform/internal/infra/runtimehttp"
	"ocp.local/platform/internal/infra/telemetry"
)

func main() {
	listen := flag.String("listen", ":8080", "HTTP listen address")
	role := flag.String("role", "public", "Process role: public, internal, or worker")
	service := flag.String("service", "", "Service composition: combined, core, food, shopping, or lodging")
	workSafety := flag.Duration("work-recovery-interval", time.Minute, "Durable worker safety scan interval (100ms–1h)")
	check := flag.Bool("healthcheck", false, "check the local API process and exit")
	flag.Parse()
	process, err := runtimeconfig.Resolve(*service, *role, os.Getenv("OCP_ENABLED_DOMAINS"))
	if err != nil {
		slog.Error("invalid service composition")
		os.Exit(1)
	}
	if *check {
		client := &http.Client{Timeout: 2 * time.Second}
		res, err := client.Get("http://127.0.0.1:8080/health/live")
		if err != nil {
			os.Exit(1)
		}
		_ = res.Body.Close() // Read-only health response; no buffered write to flush.
		if res.StatusCode != http.StatusOK {
			os.Exit(1)
		}
		return
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	config, err := telemetry.ConfigFromEnv(os.Getenv)
	if err != nil {
		slog.Error("invalid telemetry configuration")
		os.Exit(1)
	}
	telemetry.InstallErrorHandler(slog.Default())
	signals, err := telemetry.New(ctx, config, slog.Default())
	if err != nil {
		slog.Error("telemetry startup failed")
		os.Exit(1)
	}
	defer func() {
		shutdown, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if signals.Shutdown(shutdown) != nil {
			slog.Warn("telemetry shutdown incomplete")
		}
	}()
	startup, cancel := context.WithTimeout(ctx, 10*time.Second)
	pool, err := database.PoolFromEnv(os.Getenv)
	if err != nil {
		cancel()
		slog.Error("invalid database pool configuration")
		os.Exit(1)
	}
	db, err := database.OpenWithPool(startup, os.Getenv("OCP_DATABASE_URL"), pool)
	if err == nil {
		err = database.CheckRuntimeRole(startup, db)
	}
	cancel()
	if err != nil {
		slog.Error("database startup failed")
		os.Exit(1)
	}
	defer db.Close()
	if process.Role == "worker" {
		enabled, err := capability.Parse(process.Domains, []capability.Domain{capability.Food}, []capability.Domain{capability.Food})
		if err != nil || len(enabled.Enabled()) != 1 {
			slog.Error("invalid worker domains")
			os.Exit(1)
		}
		publisher, err := networkcatalog.LoadPublisher(os.Getenv("OCP_NETWORK_PUBLISHER_CONFIG"), os.Getenv("OCP_LOCAL_NETWORK") == "true")
		if err != nil {
			slog.Error("invalid publisher configuration")
			os.Exit(1)
		}
		worker := work.Worker{Store: work.SQLStore{DB: db}, Wake: work.PostgresWake{DSN: os.Getenv("OCP_DATABASE_URL"), Domains: enabled.Enabled()}, SafetyInterval: *workSafety, Domains: enabled.Enabled(), Handlers: map[string]work.Handler{"food/catalog.publish": publisher.Deliver}, Concurrency: 2, Observer: signals.Observer}
		workerCtx, stopWorker := context.WithCancel(ctx)
		defer stopWorker()
		workerDone := make(chan error, 1)
		go func() { workerDone <- worker.Run(workerCtx) }()
		mux := http.NewServeMux()
		mux.HandleFunc("GET /health/live", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"status":"live"}`))
		})
		healthDone := make(chan error, 1)
		go func() { healthDone <- serve(workerCtx, *listen, mux) }()
		select {
		case err = <-workerDone:
			stopWorker()
		case err = <-healthDone:
			stopWorker()
			select {
			case workerErr := <-workerDone:
				err = errors.Join(err, workerErr)
			case <-time.After(15 * time.Second):
				slog.Warn("worker cleanup timed out")
			}
		case <-ctx.Done():
			select {
			case workerErr := <-workerDone:
				err = errors.Join(err, workerErr)
			case <-time.After(15 * time.Second):
				slog.Warn("worker cleanup timed out")
			}
		}
		if err != nil {
			slog.Error("worker stopped")
			os.Exit(1)
		}
		return
	}
	if process.Role == "internal" {
		concurrency := 4
		if raw := os.Getenv("OCP_DOMAIN_CONCURRENCY"); raw != "" {
			concurrency, err = strconv.Atoi(raw)
			if err != nil {
				slog.Error("invalid domain concurrency")
				os.Exit(1)
			}
		}
		handler, err := ucpinternal.NewHandler(db, ucpinternal.Config{Domains: process.Domains, Endpoint: os.Getenv("OCP_INTERNAL_ENDPOINT"), Concurrency: concurrency, Observer: signals.Observer})
		if err != nil {
			slog.Error("internal contract startup failed")
			os.Exit(1)
		}
		if err := serve(ctx, *listen, handler); err != nil {
			slog.Error("internal API stopped")
			os.Exit(1)
		}
		return
	}
	var options []httpapi.Option
	options = append(options, httpapi.WithDomains(process.Domains), httpapi.WithService(process.Service))
	options = append(options, httpapi.WithTelemetry(signals.Observer))
	var storage *media.Storage
	if process.Service != "food" {
		storage, err = media.FromEnv(os.Getenv)
	}
	if err != nil {
		slog.Error("invalid media configuration")
		os.Exit(1)
	}
	if storage != nil {
		options = append(options, httpapi.WithMediaStorage(storage))
	}
	if os.Getenv("OCP_LOCAL_MERCHANT_MCP") == "true" {
		options = append(options, httpapi.WithLocalMerchantMCP())
	}
	if process.Service != "core" && process.Domains != "none" {
		updates := orderupdates.New()
		go updates.Run(ctx, os.Getenv("OCP_DATABASE_URL"))
		options = append(options, httpapi.WithOrderUpdates(updates))
	}
	if process.Service != "food" && process.Domains != "none" {
		offeringUpdates := orderupdates.NewOfferings()
		go offeringUpdates.Run(ctx, os.Getenv("OCP_DATABASE_URL"))
		options = append(options, httpapi.WithOfferingUpdates(offeringUpdates))
	}
	if os.Getenv("OCP_DEVELOPMENT_PAYMENTS") == "true" && process.Service != "core" {
		options = append(options, httpapi.WithDevelopmentPayments())
		slog.Warn("development payments enabled; no real money is moved")
	}
	handler, err := httpapi.NewHandler(db, options...)
	if err != nil {
		slog.Error("contract startup failed")
		os.Exit(1)
	}
	if err := serve(ctx, *listen, handler); err != nil {
		slog.Error("API stopped", "error", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, listen string) error {
	return serve(ctx, listen, runtimehttp.NewHandler())
}
func serve(ctx context.Context, listen string, handler http.Handler) error {
	server := &http.Server{
		Addr:              listen,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    16 << 10,
	}
	result := make(chan error, 1)
	go func() { result <- server.ListenAndServe() }()
	slog.Info("API listening", "address", listen)
	select {
	case err := <-result:
		return fmt.Errorf("serve HTTP: %w", err)
	case <-ctx.Done():
	}
	shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdown); err != nil {
		_ = server.Close()
		return fmt.Errorf("shut down HTTP: %w", err)
	}
	if err := <-result; err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serve HTTP: %w", err)
	}
	return nil
}
