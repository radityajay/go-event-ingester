package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/radityajayantara/go-event-ingester/internal/config"
	"github.com/radityajayantara/go-event-ingester/internal/handler"
	"github.com/radityajayantara/go-event-ingester/internal/metrics"
	"github.com/radityajayantara/go-event-ingester/internal/middleware"
	"github.com/radityajayantara/go-event-ingester/internal/postgres"
	redispkg "github.com/radityajayantara/go-event-ingester/internal/redis"
	"github.com/radityajayantara/go-event-ingester/internal/worker"
)

func main() {
	// Structured logging.
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	// Load config.
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	// Initialize metrics.
	m := metrics.NewCollector()

	// Initialize Redis producer.
	producer, err := redispkg.NewProducer(cfg.Redis)
	if err != nil {
		slog.Error("failed to create redis producer", "error", err)
		os.Exit(1)
	}
	defer producer.Close()

	// Initialize PostgreSQL store.
	store, err := postgres.NewStore(cfg.Postgres)
	if err != nil {
		slog.Error("failed to create postgres store", "error", err)
		os.Exit(1)
	}
	defer store.Close()

	// Start worker pool.
	pool := worker.NewPool(cfg.Worker, cfg.Redis, store, m)
	pool.Start(context.Background())

	// Setup HTTP routes.
	mux := http.NewServeMux()
	ingestHandler := handler.NewIngestHandler(producer, m)
	bp := middleware.NewBackpressure(5000) // Max 5000 concurrent requests.

	mux.HandleFunc("/api/v1/events", bp.Wrap(ingestHandler.HandleSingle))
	mux.HandleFunc("/api/v1/events/batch", bp.Wrap(ingestHandler.HandleBatch))
	mux.HandleFunc("/metrics", m.Handler())
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	server := &http.Server{
		Addr:         ":" + cfg.Server.Port,
		Handler:      mux,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	// Start server in a goroutine.
	go func() {
		slog.Info("server starting", "port", cfg.Server.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	// Graceful shutdown: wait for interrupt signal.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit

	slog.Info("shutdown signal received", "signal", sig)

	// 1. Stop accepting new HTTP requests.
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("server shutdown error", "error", err)
	}
	slog.Info("http server stopped")

	// 2. Stop worker pool (drains in-flight events).
	pool.Stop()

	slog.Info("shutdown complete")
}
