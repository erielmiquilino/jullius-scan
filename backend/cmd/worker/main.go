package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/erielfranco/jullius-scan/backend/internal/config"
	"github.com/erielfranco/jullius-scan/backend/internal/database"
	"github.com/erielfranco/jullius-scan/backend/internal/queue"
	"github.com/erielfranco/jullius-scan/backend/internal/scraper"
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

	// Worker
	w := scraper.NewWorker(db, queueClient, cfg)

	// Graceful shutdown
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh

		slog.Info("shutting down worker")
		cancel()
	}()

	slog.Info("starting scraping worker", "pool_size", cfg.WorkerPoolSize)
	w.Run(ctx)
}
