package ratelimiter

import (
	"context"
	"time"
)

type RateLimiter interface {
	Allow(ctx context.Context, key string) (bool, error)
	AllowN(ctx context.Context, key string, n int) (bool, error)
	Reset(ctx context.Context, key string) error
	GetLimit() int
	GetWindow() time.Duration
}

type Config struct {
	Limit     int
	Window    time.Duration
	BurstSize int
}

type Result struct {
	Allowed    bool
	Limit      int
	Remaining  int
	RetryAfter time.Time
	ResetAt    time.Time
}

type ClientConfig struct {
	ClientID   string
	Limit      int
	Window     time.Duration
	Attributes map[string]string
}
