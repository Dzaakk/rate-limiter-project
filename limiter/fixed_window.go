package limiter

import (
	"context"
	"fmt"
	"time"
)

type fixedWindow struct {
	redis RedisClient
	rl    *RateLimiter
}

func (f *fixedWindow) windowStart(t time.Time, window time.Duration) time.Time {
	return t.Truncate(window)
}
func (f *fixedWindow) generateKey(ClientID string, t time.Time, window time.Duration) string {
	ws := f.windowStart(t, window)
	return fmt.Sprintf("reatelimit:%s:%d", ClientID, ws.Unix())
}

func (f *fixedWindow) Allow(ctx context.Context, clientID string) (bool, int, time.Duration, error) {
	limit := f.rl.GetLimit(clientID)
	if limit.Requests <= 0 || limit.Window <= 0 {
		return false, 0, 0, fmt.Errorf("invalid limit configuration for client %s", clientID)
	}
	now := time.Now().UTC()
	key := f.generateKey(clientID, now, limit.Window)

	count, err := f.redis.Incr(ctx, key)
	if err != nil {
		return false, 0, 0, err
	}

	if count == 1 {
		_, err := f.redis.Expire(ctx, key, limit.Window)
		if err != nil {
			return false, 0, 0, err
		}
	}

	remaining := int(limit.Requests - int(count))
	if remaining < 0 {
		remaining = 0
	}

	ws := f.windowStart(now, limit.Window)
	resetIn := ws.Add(limit.Window).Sub(now)
	allowed := count <= int64(limit.Requests)
	return allowed, remaining, resetIn, nil
}
