package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/Dzaakk/rate-limiter/limiter"
)

func main() {
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		addr = "localhost:6379"
	}

	redisClient := limiter.NewRedisClient(addr)
	defaultLimit := limiter.ClientLimit{Requests: 100, Window: 1 * time.Minute}
	rl := limiter.NewRateLimiter(redisClient, defaultLimit)

	rl.SetLimit("alpha", limiter.ClientLimit{Requests: 50, Window: 30 * time.Second})

	http.HandleFunc("/api", func(w http.ResponseWriter, r *http.Request) {
		client := r.Header.Get("X-Client-ID")
		if client == "" {
			client = "bob"
		}

		allowed, remaining, resetIn, err := rl.Allow(context.Background(), client)
		if err != nil {
			log.Printf("error on allow: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, "internal error")
			return
		}

		w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))
		w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(int64(resetIn.Seconds()), 10))
		if !allowed {
			w.WriteHeader(http.StatusTooManyRequests)
			fmt.Fprintf(w, "rate limit exceeded")
			return
		}
		fmt.Fprintf(w, "hello %s\n", client)
	})

	addrServe := ":8080"
	log.Printf("listening on %s (redis=%s)", addrServe, addr)
	log.Fatal(http.ListenAndServe(addrServe, nil))
}
