package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"ocp.local/platform/internal/httpapi"
	"ocp.local/platform/internal/orderupdates"
	"ocp.local/platform/internal/persistence"
	"ocp.local/platform/internal/runtimehttp"
	"ocp.local/platform/internal/telemetry"
)

func main() {
	listen := flag.String("listen", ":8080", "HTTP listen address")
	check := flag.Bool("healthcheck", false, "check the local API process and exit")
	flag.Parse()
	if *check {
		client := &http.Client{Timeout: 2 * time.Second}
		res, err := client.Get("http://127.0.0.1:8080/health/live")
		if err != nil {
			os.Exit(1)
		}
		res.Body.Close()
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
	db, err := persistence.Open(startup, os.Getenv("OCP_DATABASE_URL"))
	if err == nil {
		err = persistence.CheckRuntimeRole(startup, db)
	}
	cancel()
	if err != nil {
		slog.Error("database startup failed")
		os.Exit(1)
	}
	defer db.Close()
	var options []httpapi.Option
	options = append(options, httpapi.WithTelemetry(signals.Observer))
	updates := orderupdates.New()
	go updates.Run(ctx, os.Getenv("OCP_DATABASE_URL"))
	options = append(options, httpapi.WithOrderUpdates(updates))
	if os.Getenv("OCP_DEVELOPMENT_PAYMENTS") == "true" {
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
