package limiter

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisClient interface {
	Incr(ctx context.Context, key string) (int64, error)
	TTL(ctx context.Context, key string) (time.Duration, error)
	Expire(ctx context.Context, key string, ttl time.Duration) (bool, error)
}

type RedisClientImpl struct {
	c *redis.Client
}

func NewRedisClient(addr string, opts ...func(*redis.Options)) *RedisClientImpl {
	ro := &redis.Options{Addr: addr}
	for _, f := range opts {
		f(ro)
	}

	r := redis.NewClient(ro)
	return &RedisClientImpl{c: r}
}

func (r *RedisClientImpl) Incr(ctx context.Context, key string) (int64, error) {
	t := r.c.Incr(ctx, key)
	return t.Result()
}

func (r *RedisClientImpl) TTL(ctx context.Context, key string) (time.Duration, error) {
	res := r.c.TTL(ctx, key)
	return res.Result()
}

func (r *RedisClientImpl) Expire(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	res := r.c.Expire(ctx, key, ttl)
	return res.Result()
}
