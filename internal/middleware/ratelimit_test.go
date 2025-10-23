package middleware

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/Dzaakk/rate-limiter/config"
	"github.com/Dzaakk/rate-limiter/internal/limiter"
	"github.com/Dzaakk/rate-limiter/internal/storage/memory"
)

type mockStoreError struct{}

func (m *mockStoreError) Increment(key string, ttl time.Duration) (int64, time.Time, error) {
	return 0, time.Time{}, errors.New("storage error")
}

func (m *mockStoreError) Get(key string) (int64, time.Time, error) {
	return 0, time.Time{}, errors.New("storage error")
}

func TestNewRateLimitMiddleware(t *testing.T) {
	store := memory.NewMemoryStore()
	l := limiter.NewLimiter(store, config.Clients)
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	mw := NewRateLimitMiddleware(l, logger)

	if mw == nil {
		t.Fatal("expected middleware to be created")
	}
	if mw.limiter != l {
		t.Fatal("expected limiter to be set")
	}
	if mw.logger != logger {
		t.Fatal("expected logger to be set")
	}
}

func TestGetClientID(t *testing.T) {
	store := memory.NewMemoryStore()
	l := limiter.NewLimiter(store, config.Clients)
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	mw := NewRateLimitMiddleware(l, logger)

	tests := []struct {
		name       string
		headerVal  string
		wantClient string
	}{
		{
			name:       "with client ID header",
			headerVal:  "client-1",
			wantClient: "client-1",
		},
		{
			name:       "without client ID header",
			headerVal:  "",
			wantClient: "default",
		},
		{
			name:       "with custom client ID",
			headerVal:  "my-custom-client",
			wantClient: "my-custom-client",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test", nil)
			if tt.headerVal != "" {
				req.Header.Set("X-Client-ID", tt.headerVal)
			}

			clientID := mw.getClientID(req)
			if clientID != tt.wantClient {
				t.Errorf("expected client ID %s, got %s", tt.wantClient, clientID)
			}
		})
	}
}

func TestGetLimit(t *testing.T) {
	store := memory.NewMemoryStore()
	l := limiter.NewLimiter(store, config.Clients)
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	mw := NewRateLimitMiddleware(l, logger)

	tests := []struct {
		name      string
		clientID  string
		wantLimit int
	}{
		{
			name:      "configured client-1",
			clientID:  "client-1",
			wantLimit: 5,
		},
		{
			name:      "configured client-2",
			clientID:  "client-2",
			wantLimit: 2,
		},
		{
			name:      "unknown client uses default",
			clientID:  "unknown",
			wantLimit: 100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			limit := mw.getLimit(tt.clientID)
			if limit != tt.wantLimit {
				t.Errorf("expected limit %d, got %d", tt.wantLimit, limit)
			}
		})
	}
}

func TestRateLimitMiddleware_Handler_Success(t *testing.T) {
	store := memory.NewMemoryStore()
	l := limiter.NewLimiter(store, config.Clients)
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	mw := NewRateLimitMiddleware(l, logger)

	handlerCalled := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Client-ID", "client-1")
	rec := httptest.NewRecorder()

	mw.Handler(handler)(rec, req)

	if !handlerCalled {
		t.Fatal("expected handler to be called")
	}

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	limitHeader := rec.Header().Get("X-RateLimit-Limit")
	if limitHeader != "5" {
		t.Errorf("expected limit header '5', got '%s'", limitHeader)
	}

	remainingHeader := rec.Header().Get("X-RateLimit-Remaining")
	remaining, err := strconv.Atoi(remainingHeader)
	if err != nil {
		t.Fatalf("failed to parse remaining header: %v", err)
	}
	if remaining != 4 {
		t.Errorf("expected remaining 4, got %d", remaining)
	}

	resetHeader := rec.Header().Get("X-RateLimit-Reset")
	if resetHeader == "" {
		t.Error("expected reset header to be set")
	}
}

func TestRateLimitMiddleware_Handler_RateLimitExceeded(t *testing.T) {
	store := memory.NewMemoryStore()
	cfgs := map[string]config.ClientConfig{
		"test-client": {Limit: 2, Window: time.Minute},
	}
	l := limiter.NewLimiter(store, cfgs)
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	mw := NewRateLimitMiddleware(l, logger)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("X-Client-ID", "test-client")
		rec := httptest.NewRecorder()

		mw.Handler(handler)(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("request %d: expected status 200, got %d", i+1, rec.Code)
		}
	}

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Client-ID", "test-client")
	rec := httptest.NewRecorder()

	mw.Handler(handler)(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("expected status 429, got %d", rec.Code)
	}

	remainingHeader := rec.Header().Get("X-RateLimit-Remaining")
	if remainingHeader != "0" {
		t.Errorf("expected remaining '0', got '%s'", remainingHeader)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response["error"] != "Rate limit exceeded" {
		t.Errorf("expected error message, got %v", response["error"])
	}

	if response["remaining"] != float64(0) {
		t.Errorf("expected remaining 0, got %v", response["remaining"])
	}
}

func TestRateLimitMiddleware_Handler_StorageError(t *testing.T) {
	l := limiter.NewLimiter(&mockStoreError{}, config.Clients)
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	mw := NewRateLimitMiddleware(l, logger)

	handlerCalled := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Client-ID", "client-1")
	rec := httptest.NewRecorder()

	mw.Handler(handler)(rec, req)

	if handlerCalled {
		t.Fatal("expected handler not to be called on storage error")
	}

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", rec.Code)
	}
}

func TestRateLimitMiddleware_Handler_Concurrent(t *testing.T) {
	store := memory.NewMemoryStore()
	cfgs := map[string]config.ClientConfig{
		"concurrent-client": {Limit: 100, Window: time.Minute},
	}
	l := limiter.NewLimiter(store, cfgs)
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	mw := NewRateLimitMiddleware(l, logger)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	N := 50
	results := make(chan int, N)

	for i := 0; i < N; i++ {
		go func() {
			req := httptest.NewRequest("GET", "/test", nil)
			req.Header.Set("X-Client-ID", "concurrent-client")
			rec := httptest.NewRecorder()

			mw.Handler(handler)(rec, req)
			results <- rec.Code
		}()
	}

	successCount := 0
	for i := 0; i < N; i++ {
		code := <-results
		if code == http.StatusOK {
			successCount++
		}
	}

	if successCount != N {
		t.Errorf("expected %d successful requests, got %d", N, successCount)
	}
}
