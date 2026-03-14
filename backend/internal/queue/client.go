package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	// JobQueueKey is the Redis list key for pending scraping jobs.
	JobQueueKey = "jullius:scraping:jobs"
)

// JobMessage represents a scraping job message in the queue.
type JobMessage struct {
	JobID     int64  `json:"job_id"`
	FiscalURL string `json:"fiscal_url"`
	HouseID   int64  `json:"house_id"`
	Attempt   int    `json:"attempt"`
}

// Client wraps the Redis client for queue operations.
type Client struct {
	rdb *redis.Client
}

// NewClient creates a new Redis queue client.
func NewClient(addr, password string, db int) *Client {
	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})
	return &Client{rdb: rdb}
}

// Close closes the Redis connection.
func (c *Client) Close() error {
	return c.rdb.Close()
}

// Ping checks Redis connectivity.
func (c *Client) Ping(ctx context.Context) error {
	return c.rdb.Ping(ctx).Err()
}

// Enqueue pushes a scraping job to the queue.
func (c *Client) Enqueue(ctx context.Context, msg JobMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal job message: %w", err)
	}
	return c.rdb.LPush(ctx, JobQueueKey, data).Err()
}

// Dequeue blocks waiting for a scraping job from the queue.
// Returns the job message or blocks until timeout.
func (c *Client) Dequeue(ctx context.Context, timeout time.Duration) (*JobMessage, error) {
	result, err := c.rdb.BRPop(ctx, timeout, JobQueueKey).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil // timeout, no job available
		}
		return nil, fmt.Errorf("dequeue job: %w", err)
	}

	if len(result) < 2 {
		return nil, fmt.Errorf("unexpected BRPop result length: %d", len(result))
	}

	var msg JobMessage
	if err := json.Unmarshal([]byte(result[1]), &msg); err != nil {
		return nil, fmt.Errorf("unmarshal job message: %w", err)
	}

	return &msg, nil
}
