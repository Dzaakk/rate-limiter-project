package memory

import (
	"sync"
	"sync/atomic"
	"time"
)

type Entry struct {
	Count  int64
	Expiry time.Time
}

type MemoryStore struct {
	mu sync.RWMutex
	m  map[string]*Entry
}

func NewMemoryStore() *MemoryStore {
	s := &MemoryStore{m: map[string]*Entry{}}
	go s.cleanupLoop()

	return s
}

func (s *MemoryStore) cleanupLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		now := time.Now()
		s.mu.Lock()
		for k, e := range s.m {
			if e == nil {
				delete(s.m, k)
				continue
			}
			if e.Expiry.Before(now) {
				delete(s.m, k)
			}
		}
		s.mu.Unlock()
	}
}

func (s *MemoryStore) Increment(key string, ttl time.Duration) (int64, time.Time, error) {
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()

	e, ok := s.m[key]
	if !ok || e == nil || e.Expiry.Before(now) { //create new entry

		e = &Entry{Count: 1, Expiry: now.Add(ttl)}
		s.m[key] = e

		return 1, e.Expiry, nil
	}

	newv := atomic.AddInt64(&e.Count, 1)
	return newv, e.Expiry, nil
}

func (s *MemoryStore) Get(key string) (int64, time.Time, error) {
	now := time.Now()
	s.mu.RLock()
	e, ok := s.m[key]
	s.mu.RUnlock()
	if !ok || e == nil || e.Expiry.Before(now) {
		return 0, time.Time{}, nil
	}

	return atomic.LoadInt64(&e.Count), e.Expiry, nil
}
