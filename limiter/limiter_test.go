package limiter

import (
	"context"
	"sync"
	"testing"
	"time"
)

type mockRedis struct {
	mu    sync.Mutex
	store map[string]struct {
		count int64
		ttl   time.Time
	}
}

func newMockRedis() *mockRedis {
	return &mockRedis{store: make(map[string]struct {
		count int64
		ttl   time.Time
	})}
}

func (m *mockRedis) Incr(ctx context.Context, key string) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	s := m.store[key]
	if !s.ttl.IsZero() && time.Now().After(s.ttl) {
		s = struct {
			count int64
			ttl   time.Time
		}{}
	}
	s.count++
	m.store[key] = s

	return s.count, nil
}

func (m *mockRedis) TTL(ctx context.Context, key string) (time.Duration, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	s := m.store[key]
	if s.ttl.IsZero() {
		return -1, nil
	}

	return time.Until(s.ttl), nil
}

func (m *mockRedis) Expire(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	s := m.store[key]
	s.ttl = time.Now().Add(ttl)
	m.store[key] = s

	return true, nil
}

func TestFixedWindowAllowBasic(t *testing.T) {
	mr := newMockRedis()
	rl := NewRateLimiter(mr, ClientLimit{Requests: 2, Window: 1 * time.Second})

	allowed, remaining, reset, err := rl.Allow(context.Background(), "c1")

	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if !allowed {
		t.Fatalf("first request should be allowed")
	}

	if remaining != 1 {
		t.Fatalf("remaining should be 1, got %d", remaining)
	}

	if reset <= 0 {
		t.Fatalf("reset should be > 0")
	}

	allowed, remaining, _, _ = rl.Allow(context.Background(), "c1")
	if !allowed {
		t.Fatalf("second request should be allowed")
	}
	if remaining != 0 {
		t.Fatalf("remaining should be 0, got %d", remaining)
	}

	allowed, _, _, _ = rl.Allow(context.Background(), "c1")
	if allowed {
		t.Fatalf("third request should be blocked")
	}
}

func TestPerClientLimits(t *testing.T) {
	mr := newMockRedis()
	rl := NewRateLimiter(mr, ClientLimit{Requests: 100, Window: 1 * time.Minute})
	rl.SetLimit("special", ClientLimit{Requests: 1, Window: 1 * time.Second})

	allowed, _, _, _ := rl.Allow(context.Background(), "special")
	if !allowed {
		t.Fatalf("first request allowed")
	}

	allowed, _, _, _ = rl.Allow(context.Background(), "special")
	if allowed {
		t.Fatalf("first request blocked")
	}
}

func TestConcurrency(t *testing.T) {
	mr := newMockRedis()
	rl := NewRateLimiter(mr, ClientLimit{Requests: 50, Window: 1 * time.Second})

	var wg sync.WaitGroup
	allowedCount := 0
	mu := sync.Mutex{}

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ok, _, _, _ := rl.Allow(context.Background(), "hot")

			if ok {
				mu.Lock()
				allowedCount++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if allowedCount != 50 {
		t.Fatalf("expected 50 allowed, got %d", allowedCount)
	}
}
