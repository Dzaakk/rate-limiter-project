package memory

import (
	"sync"
	"testing"
	"time"
)

func TestMemoryStoreIncrementAndGet(t *testing.T) {
	s := NewMemoryStore()
	key := "foo:1"

	counter, exp, err := s.Increment(key, 100*time.Millisecond)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if counter != 1 {
		t.Fatalf("expected 1 got %d", counter)
	}

	counter2, exp2, _ := s.Increment(key, 100*time.Millisecond)
	if counter2 != 2 {
		t.Fatalf("expected 2 got %d", counter2)
	}
	if !exp2.Equal(exp) {
		t.Fatal("expected same expiry to be preserved")
	}

	time.Sleep(150 * time.Millisecond)
	counter3, _, _ := s.Get(key)
	if counter3 != 0 {
		t.Fatalf("expected 0 after expiry got %d", counter3)
	}
}

func TestMemoryStoreConcurrency(t *testing.T) {
	s := NewMemoryStore()
	key := "concurrent:1"
	ttl := 1 * time.Second

	var wg sync.WaitGroup
	N := 100
	wg.Add(N)

	for i := 0; i < N; i++ {
		go func() {
			defer wg.Done()
			s.Increment(key, ttl)
		}()
	}
	wg.Wait()
	counter, _, _ := s.Get(key)
	if counter != int64(N) {
		t.Fatalf("expected %d got %d", N, counter)
	}

}
