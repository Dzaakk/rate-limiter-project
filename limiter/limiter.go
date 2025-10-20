package limiter

import (
	"context"
	"sync"
	"time"
)

type Algorithm interface {
	Allow(ctx context.Context, clientID string) (allowed bool, remaining int, resetIn time.Duration, err error)
}

type ClientLimit struct {
	Requests int
	Window   time.Duration
}

type RateLimiter struct {
	redis        RedisClient
	mu           sync.RWMutex
	clientLimits map[string]ClientLimit
	defaultLimit ClientLimit
	impl         Algorithm
}

func NewRateLimiter(redis RedisClient, defaultLimit ClientLimit) *RateLimiter {
	r := &RateLimiter{
		redis:        redis,
		clientLimits: make(map[string]ClientLimit),
		defaultLimit: defaultLimit,
	}

	r.impl = &fixedWindow{
		redis: r.redis,
		rl:    r,
	}

	return r
}

func (r *RateLimiter) SetLimit(clientID string, limit ClientLimit) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.clientLimits[clientID] = limit
}

func (r *RateLimiter) GetLimit(clientID string) ClientLimit {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if v, ok := r.clientLimits[clientID]; ok {
		return v
	}
	return r.defaultLimit
}

func (r *RateLimiter) Allow(ctx context.Context, clientID string) (bool, int, time.Duration, error) {
	return r.impl.Allow(ctx, clientID)
}
