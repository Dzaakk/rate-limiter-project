package limiter

import (
	"errors"
	"testing"
	"time"

	"github.com/Dzaakk/rate-limiter/config"
	"github.com/Dzaakk/rate-limiter/internal/storage/memory"
)

type mockStoreError struct{}

func (m *mockStoreError) Increment(key string, ttl time.Duration) (int64, time.Time, error) {
	return 0, time.Time{}, errors.New("mock increment error")
}
func (m *mockStoreError) Get(key string) (int64, time.Time, error) {
	return 0, time.Time{}, errors.New("mock get error")
}

type mockStorePastExpiry struct {
	count int64
}

func (m *mockStorePastExpiry) Increment(key string, ttl time.Duration) (int64, time.Time, error) {
	return m.count + 1, time.Now().Add(-1 * time.Second), nil
}
func (m *mockStorePastExpiry) Get(key string) (int64, time.Time, error) {
	return m.count, time.Now().Add(-1 * time.Second), nil
}

func TestAllow(t *testing.T) {
	cfgs := map[string]config.ClientConfig{"c1": {Limit: 3, Window: time.Second}}

	t.Run("uses default config when client not found", func(t *testing.T) {
		l := NewLimiter(memory.NewMemoryStore(), map[string]config.ClientConfig{})
		ok, _, _, _ := l.Allow("unknown-client")
		if !ok {
			t.Fatal("expected allowed under default config")
		}
	})
	t.Run("error store increment", func(t *testing.T) {
		l := NewLimiter(&mockStoreError{}, cfgs)
		ok, remaining, resetAt, err := l.Allow("c1")
		if err == nil {
			t.Fatal("expected error")
		}
		if !ok || remaining != cfgs["c1"].Limit || !resetAt.IsZero() {
			t.Fatalf("unexpected response on store error")
		}
	})
	t.Run("remaining less than 0", func(t *testing.T) {
		s := memory.NewMemoryStore()
		l := NewLimiter(s, cfgs)
		for i := 0; i < 3; i++ {
			ok, remaining, resetAt, err := l.Allow("c1")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !ok {
				t.Fatalf("expected allowed on iteration %d", i)
			}
			if remaining != 3-(i+1) {
				t.Fatalf("unexpected remaining: %d", remaining)
			}
			if resetAt.IsZero() {
				t.Fatal("expected resetAt to be set")
			}
		}

		ok, remaining, _, _ := l.Allow("c1")
		if ok {
			t.Fatal("expected denied on 4th")
		}
		if remaining != 0 {
			t.Fatalf("expected remaining 0 got %d", remaining)
		}
	})
	t.Run("expiry before now", func(t *testing.T) {
		l := NewLimiter(&mockStorePastExpiry{}, cfgs)
		ok, _, resetAt, _ := l.Allow("c1")
		if !ok || !resetAt.IsZero() {
			t.Fatalf("expected allowed with zero resetAt")
		}
	})
}

func TestLimiterConcurrency(t *testing.T) {
	s := memory.NewMemoryStore()
	cfgs := map[string]config.ClientConfig{"c2": {Limit: 100, Window: time.Second}}
	l := NewLimiter(s, cfgs)
	N := 100
	ch := make(chan bool, N)

	for i := 0; i < N; i++ {
		go func() {
			ok, _, _, _ := l.Allow("c2")
			ch <- ok
		}()
	}

	allowedCount := 0
	for i := 0; i < N; i++ {
		if <-ch {
			allowedCount++
		}
	}
	if allowedCount != N {
		t.Fatalf("expected %d allowed got %d", N, allowedCount)
	}
}
