package main

import (
	"context"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Dzaakk/rate-limiter/config"
	"github.com/Dzaakk/rate-limiter/internal/handler"
	"github.com/Dzaakk/rate-limiter/internal/limiter"
	"github.com/Dzaakk/rate-limiter/internal/middleware"
	"github.com/Dzaakk/rate-limiter/internal/storage/memory"
	"github.com/Dzaakk/rate-limiter/internal/storage/redis"
	goredis "github.com/redis/go-redis/v9"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	store := initStorage(logger)

	l := limiter.NewLimiter(store, config.Clients)

	rateLimitMW := middleware.NewRateLimitMiddleware(l, logger)

	mux := http.NewServeMux()
	mux.HandleFunc("/api/hello", rateLimitMW.Handler(handler.HelloHandler))
	mux.HandleFunc("/api/status", handler.StatusHandler)

	httpServer := &http.Server{
		Addr:         ":8080",
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		logger.Info("starting HTTP server", "addr", httpServer.Addr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server error", "error", err)
			log.Fatal(err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		logger.Error("server forced to shutdown", "error", err)
		log.Fatal(err)
	}

	logger.Info("server stopped")
}

func initStorage(logger *slog.Logger) limiter.Store {
	storageType := os.Getenv("STORAGE_TYPE")
	if storageType == "" {
		storageType = "memory"
	}

	switch storageType {
	case "redis":
		return initRedisStorage(logger)
	default:
		logger.Info("using in-memory storage")
		return memory.NewMemoryStore()
	}
}

func initRedisStorage(logger *slog.Logger) limiter.Store {
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}

	logger.Info("connecting to Redis", "addr", redisAddr)
	rdb := goredis.NewClient(&goredis.Options{
		Addr: redisAddr,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		logger.Error("failed to connect to Redis", "error", err)
		log.Fatal(err)
	}

	logger.Info("successfully connected to Redis")
	return redis.NewRedisStore(rdb)
}
