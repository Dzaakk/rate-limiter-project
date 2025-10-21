package ratelimiter

import (
	"context"
	"sync"
	"time"
)

type FixedWindowLimiter struct {
	config   Config
	mu       sync.RWMutex
	windows  map[string]*windowData
	cleanMu  sync.Mutex
	stopChan chan struct{}
}

type windowData struct {
	count     int
	startTime time.Time
	mu        sync.Mutex
}

func NewFixedWindowLimiter(config Config) *FixedWindowLimiter {
	limiter := &FixedWindowLimiter{
		config:   config,
		windows:  make(map[string]*windowData),
		stopChan: make(chan struct{}),
	}

	go limiter.cleanup()

	return limiter
}

func (l *FixedWindowLimiter) Allow(ctx context.Context, key string) (bool, error) {
	return l.AllowN(ctx, key, 1)
}

func (l *FixedWindowLimiter) AllowN(ctx context.Context, key string, n int) (bool, error) {
	now := time.Now()

	l.mu.RLock()
	wd, exists := l.windows[key]
	l.mu.RUnlock()

	if !exists {
		l.mu.Lock()

		if wd, exists = l.windows[key]; !exists {
			wd = &windowData{
				startTime: now,
				count:     0,
			}
			l.windows[key] = wd
		}
		l.mu.Unlock()
	}

	wd.mu.Lock()
	defer wd.mu.Unlock()

	if now.Sub(wd.startTime) >= l.config.Window {
		wd.startTime = now
		wd.count = 0
	}

	if wd.count+n <= l.config.Limit {
		wd.count += n
		return true, nil
	}

	return false, nil
}

func (l *FixedWindowLimiter) Reset(ctx context.Context, key string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	delete(l.windows, key)
	return nil
}

func (l *FixedWindowLimiter) GetLimit() int {
	return l.config.Limit
}

func (l *FixedWindowLimiter) GetWindow() int {
	return int(l.config.Window)
}

func (l *FixedWindowLimiter) GetResult(ctx context.Context, key string) (*Result, error) {
	now := time.Now()

	l.mu.RLock()
	wd, exists := l.windows[key]
	l.mu.RUnlock()

	if !exists {
		return &Result{
			Allowed:   true,
			Limit:     l.config.Limit,
			Remaining: l.config.Limit,
			ResetAt:   now.Add(l.config.Window),
		}, nil
	}

	wd.mu.Lock()
	defer wd.mu.Unlock()

	if now.Sub(wd.startTime) >= l.config.Window {
		return &Result{
			Allowed:   true,
			Limit:     l.config.Limit,
			Remaining: l.config.Limit,
			ResetAt:   now.Add(l.config.Window),
		}, nil
	}

	remaining := l.config.Limit - wd.count
	if remaining < 0 {
		remaining = 0
	}

	resetAt := wd.startTime.Add(l.config.Window)

	return &Result{
		Allowed:    wd.count < l.config.Limit,
		Limit:      l.config.Limit,
		Remaining:  l.config.Limit,
		RetryAfter: resetAt,
		ResetAt:    resetAt,
	}, nil
}

func (l *FixedWindowLimiter) cleanup() {
	ticker := time.NewTicker(l.config.Window)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			l.cleanupExpired()
		case <-l.stopChan:
			return
		}
	}
}

func (l *FixedWindowLimiter) cleanupExpired() {
	now := time.Now()

	l.mu.Lock()
	defer l.mu.Unlock()

	for key, wd := range l.windows {
		wd.mu.Lock()
		if now.Sub(wd.startTime) >= l.config.Window*2 {
			delete(l.windows, key)
		}
		wd.mu.Unlock()
	}
}

func (l *FixedWindowLimiter) Close() {
	close(l.stopChan)
}
