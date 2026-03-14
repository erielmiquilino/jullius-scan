package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/erielfranco/jullius-scan/backend/internal/api"
	"github.com/erielfranco/jullius-scan/backend/internal/api/middleware"
	"github.com/erielfranco/jullius-scan/backend/internal/config"
	"github.com/erielfranco/jullius-scan/backend/internal/database"
	"github.com/erielfranco/jullius-scan/backend/internal/queue"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Run database migrations
	migrationsPath := getEnv("MIGRATIONS_PATH", "./migrations")
	if err := database.RunMigrations(cfg.DatabaseURL, migrationsPath); err != nil {
		slog.Error("failed to run migrations", "error", err)
		os.Exit(1)
	}

	// Database
	db, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	// Redis queue client
	queueClient := queue.NewClient(cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB)
	defer queueClient.Close()

	// Firebase Auth middleware
	authMiddleware, err := middleware.NewFirebaseAuth(ctx, cfg.FirebaseProjectID)
	if err != nil {
		slog.Error("failed to initialize firebase auth", "error", err)
		os.Exit(1)
	}

	// Router
	router := api.NewRouter(db, queueClient, authMiddleware)

	srv := &http.Server{
		Addr:         ":" + cfg.APIPort,
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh

		slog.Info("shutting down API server")
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer shutdownCancel()

		if err := srv.Shutdown(shutdownCtx); err != nil {
			slog.Error("server shutdown error", "error", err)
		}
		cancel()
	}()

	slog.Info("starting API server", "port", cfg.APIPort)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("server failed", "error", err)
		os.Exit(1)
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
