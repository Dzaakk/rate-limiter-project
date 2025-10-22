package limiter

import (
	"fmt"
	"time"

	"github.com/Dzaakk/rate-limiter/config"
)

type Store interface {
	Increment(key string, ttl time.Duration) (int64, time.Time, error)
	Get(key string) (int64, time.Time, error)
}

type Limiter struct {
	store   Store
	configs map[string]config.ClientConfig
}

func NewLimiter(s Store, cfgs map[string]config.ClientConfig) *Limiter {
	return &Limiter{store: s, configs: cfgs}
}

func keyForClient(client string) string {
	return fmt.Sprintf("rate:%s", client)
}

func (l *Limiter) Allow(client string) (bool, int, time.Time, error) {
	cfg, ok := l.configs[client]
	if !ok {
		cfg = config.DefaultConfig
	}

	now := time.Now()
	key := keyForClient(client)
	ttl := cfg.Window

	counter, expiry, err := l.store.Increment(key, ttl)
	if err != nil {
		return true, cfg.Limit, time.Time{}, err
	}

	allowed := counter <= int64(cfg.Limit)
	remaining := cfg.Limit - int(counter)
	if remaining < 0 {
		remaining = 0
	}

	if expiry.Before(now) {
		return allowed, remaining, time.Time{}, nil
	}

	return allowed, remaining, expiry, nil
}
