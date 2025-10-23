package handler

import (
	"encoding/json"
	"net/http"
	"time"
)

func HelloHandler(w http.ResponseWriter, r *http.Request) {
	clientID := r.Header.Get("X-Client-ID")
	if clientID == "" {
		clientID = "default"
	}

	response := map[string]string{
		"message":   "Hello! Your request was successful.",
		"client_id": clientID,
		"timestamp": time.Now().Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func StatusHandler(w http.ResponseWriter, r *http.Request) {
	response := map[string]string{
		"status": "ok",
		"time":   time.Now().Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}
