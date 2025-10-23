package middleware

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/Dzaakk/rate-limiter/config"
	"github.com/Dzaakk/rate-limiter/internal/limiter"
)

type RateLimitMiddleware struct {
	limiter *limiter.Limiter
	logger  *slog.Logger
}

func NewRateLimitMiddleware(l *limiter.Limiter, logger *slog.Logger) *RateLimitMiddleware {
	return &RateLimitMiddleware{
		limiter: l,
		logger:  logger,
	}
}

func (m *RateLimitMiddleware) Handler(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		clientID := m.getClientID(r)

		allowed, remaining, resetAt, err := m.limiter.Allow(clientID)
		if err != nil {
			m.logger.Error("rate limiter error", "error", err, "client", clientID)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		m.setRateLimitHeaders(w, clientID, remaining, resetAt)

		if !allowed {
			m.logger.Warn("rate limit exceeded",
				"client", clientID,
				"remaining", remaining,
				"path", r.URL.Path,
			)

			m.sendRateLimitError(w, remaining, resetAt)
			return
		}

		m.logger.Info("request allowed",
			"client", clientID,
			"remaining", remaining,
			"path", r.URL.Path,
		)

		next(w, r)
	}
}

func (m *RateLimitMiddleware) getClientID(r *http.Request) string {
	clientID := r.Header.Get("X-Client-ID")
	if clientID == "" {
		clientID = "default"
	}
	return clientID
}

func (m *RateLimitMiddleware) setRateLimitHeaders(w http.ResponseWriter, clientID string, remaining int, resetAt time.Time) {
	limit := m.getLimit(clientID)

	w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", limit))
	w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))

	if !resetAt.IsZero() {
		w.Header().Set("X-RateLimit-Reset", fmt.Sprintf("%d", resetAt.Unix()))
	}
}

func (m *RateLimitMiddleware) getLimit(clientID string) int {
	if cfg, ok := config.Clients[clientID]; ok {
		return cfg.Limit
	}
	return config.DefaultConfig.Limit
}

func (m *RateLimitMiddleware) sendRateLimitError(w http.ResponseWriter, remaining int, resetAt time.Time) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusTooManyRequests)

	response := map[string]interface{}{
		"error":     "Rate limit exceeded",
		"remaining": remaining,
	}

	if !resetAt.IsZero() {
		response["reset_at"] = resetAt.Unix()
	}

	json.NewEncoder(w).Encode(response)
}
