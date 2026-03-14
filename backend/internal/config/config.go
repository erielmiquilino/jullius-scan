package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds all application configuration loaded from environment variables.
type Config struct {
	// Server
	APIPort string

	// Database
	DatabaseURL string

	// Redis
	RedisAddr     string
	RedisPassword string
	RedisDB       int

	// Firebase
	FirebaseProjectID string

	// Scraper
	ScrapeTimeout  time.Duration
	MaxRetries     int
	WorkerPoolSize int
}

// Load reads configuration from environment variables with sensible defaults.
func Load() (*Config, error) {
	cfg := &Config{
		APIPort:           getEnv("API_PORT", "8080"),
		DatabaseURL:       getEnv("DATABASE_URL", "postgres://jullius:jullius@localhost:5432/jullius?sslmode=disable"),
		RedisAddr:         getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword:     getEnv("REDIS_PASSWORD", ""),
		FirebaseProjectID: getEnv("FIREBASE_PROJECT_ID", ""),
		WorkerPoolSize:    1,
	}

	redisDB, err := strconv.Atoi(getEnv("REDIS_DB", "0"))
	if err != nil {
		return nil, fmt.Errorf("invalid REDIS_DB: %w", err)
	}
	cfg.RedisDB = redisDB

	timeoutSec, err := strconv.Atoi(getEnv("SCRAPE_TIMEOUT_SECONDS", "45"))
	if err != nil {
		return nil, fmt.Errorf("invalid SCRAPE_TIMEOUT_SECONDS: %w", err)
	}
	cfg.ScrapeTimeout = time.Duration(timeoutSec) * time.Second

	maxRetries, err := strconv.Atoi(getEnv("MAX_RETRIES", "3"))
	if err != nil {
		return nil, fmt.Errorf("invalid MAX_RETRIES: %w", err)
	}
	cfg.MaxRetries = maxRetries

	poolSize, err := strconv.Atoi(getEnv("WORKER_POOL_SIZE", "1"))
	if err != nil {
		return nil, fmt.Errorf("invalid WORKER_POOL_SIZE: %w", err)
	}
	cfg.WorkerPoolSize = poolSize

	if cfg.FirebaseProjectID == "" {
		return nil, fmt.Errorf("FIREBASE_PROJECT_ID is required")
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
