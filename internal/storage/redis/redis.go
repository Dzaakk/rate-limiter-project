package redis

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisStore struct {
	client *redis.Client
}

func NewRedisStore(client *redis.Client) *RedisStore {
	return &RedisStore{client: client}
}

func (r *RedisStore) Increment(key string, ttl time.Duration) (int64, time.Time, error) {
	ctx := context.Background()
	now := time.Now()

	pipe := r.client.Pipeline()

	incrCmd := pipe.Incr(ctx, key)

	ttlCmd := pipe.TTL(ctx, key)

	_, err := pipe.Exec(ctx)
	if err != nil {
		return 0, time.Time{}, fmt.Errorf("redis pipeline error: %w", err)
	}

	counter := incrCmd.Val()
	currentTTL := ttlCmd.Val()

	if currentTTL == -1 || currentTTL == -2 {
		if err := r.client.Expire(ctx, key, ttl).Err(); err != nil {
			return counter, time.Time{}, fmt.Errorf("redis expire error: %w", err)
		}
		return counter, now.Add(ttl), nil
	}

	expiry := now.Add(currentTTL)
	return counter, expiry, nil
}

func (r *RedisStore) Get(key string) (int64, time.Time, error) {
	ctx := context.Background()
	now := time.Now()

	pipe := r.client.Pipeline()

	getCmd := pipe.Get(ctx, key)
	ttlCmd := pipe.TTL(ctx, key)

	_, err := pipe.Exec(ctx)
	if err == redis.Nil {
		return 0, time.Time{}, nil
	}
	if err != nil {
		return 0, time.Time{}, fmt.Errorf("redis pipeline error: %w", err)
	}

	counterStr := getCmd.Val()
	counter, err := strconv.ParseInt(counterStr, 10, 64)
	if err != nil {
		return 0, time.Time{}, fmt.Errorf("parse counter error: %w", err)
	}

	currentTTL := ttlCmd.Val()
	if currentTTL <= 0 {
		return 0, time.Time{}, nil
	}

	expiry := now.Add(currentTTL)
	return counter, expiry, nil
}
