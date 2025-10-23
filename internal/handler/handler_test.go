package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHelloHandler(t *testing.T) {
	tests := []struct {
		name           string
		clientID       string
		expectedClient string
	}{
		{
			name:           "with client ID",
			clientID:       "client-1",
			expectedClient: "client-1",
		},
		{
			name:           "without client ID",
			clientID:       "",
			expectedClient: "default",
		},
		{
			name:           "with custom client ID",
			clientID:       "my-app",
			expectedClient: "my-app",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/hello", nil)
			if tt.clientID != "" {
				req.Header.Set("X-Client-ID", tt.clientID)
			}
			rec := httptest.NewRecorder()

			HelloHandler(rec, req)

			if rec.Code != http.StatusOK {
				t.Errorf("expected status 200, got %d", rec.Code)
			}

			if rec.Header().Get("Content-Type") != "application/json" {
				t.Errorf("expected Content-Type application/json, got %s", rec.Header().Get("Content-Type"))
			}

			var response map[string]string
			if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}

			if response["message"] != "Hello! Your request was successful." {
				t.Errorf("unexpected message: %s", response["message"])
			}

			if response["client_id"] != tt.expectedClient {
				t.Errorf("expected client_id %s, got %s", tt.expectedClient, response["client_id"])
			}

			if response["timestamp"] == "" {
				t.Error("expected timestamp to be set")
			}
		})
	}
}

func TestStatusHandler(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/status", nil)
	rec := httptest.NewRecorder()

	StatusHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	if rec.Header().Get("Content-Type") != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", rec.Header().Get("Content-Type"))
	}

	var response map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response["status"] != "ok" {
		t.Errorf("expected status ok, got %s", response["status"])
	}

	if response["time"] == "" {
		t.Error("expected time to be set")
	}
}
