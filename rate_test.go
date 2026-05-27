package rate

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestOptions(t *testing.T) {
	l := New(time.Second, WithBurst(10), WithCleanCutoff(time.Second*3), WithCleanInterval(time.Second*3), WithBucketName("test"))
	defer l.Close()

	if l.burst != 10 {
		t.Errorf("burst = %d; want 10", l.burst)
	}
	if l.cleanCutoff != time.Second*3 {
		t.Errorf("cutoff = %s; want 3s", l.cleanCutoff)
	}
	if l.cleanInterval != time.Second*3 {
		t.Errorf("interval = %s; want 3s", l.cleanInterval)
	}
	if l.bucketName != "test" {
		t.Errorf("bucketName = %s; want test", l.bucketName)
	}
}

func TestRateLimiting(t *testing.T) {
	l := New(time.Millisecond*100, WithBurst(2))
	defer l.Close()

	key := "user1"

	// First two should be allowed
	if !l.Allow(key) {
		t.Errorf("expected first request to be allowed")
	}
	if !l.Allow(key) {
		t.Errorf("expected second request to be allowed")
	}

	// Third should be blocked
	if l.Allow(key) {
		t.Errorf("expected third request to be blocked")
	}

	// Wait for a bit
	time.Sleep(time.Millisecond * 150)

	// Should be allowed again
	if !l.Allow(key) {
		t.Errorf("expected request to be allowed after wait")
	}
}

func TestWait(t *testing.T) {
	l := New(time.Millisecond*100, WithBurst(1))
	defer l.Close()

	key := "user1"

	// First allowed
	l.Allow(key)

	// Second should wait
	start := time.Now()
	err := l.Wait(context.Background(), key)
	if err != nil {
		t.Fatalf("Wait failed: %v", err)
	}

	duration := time.Since(start)
	if duration < time.Millisecond*100 {
		t.Errorf("Wait returned too early: %v", duration)
	}
}

func TestMiddleware(t *testing.T) {
	l := New(time.Hour, WithBurst(1)) // Very slow rate
	defer l.Close()

	handler := l.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}), nil)

	// First request: OK
	req1 := httptest.NewRequest("GET", "/", nil)
	rr1 := httptest.NewRecorder()
	handler.ServeHTTP(rr1, req1)
	if rr1.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr1.Code)
	}

	// Second request: 429
	req2 := httptest.NewRequest("GET", "/", nil)
	rr2 := httptest.NewRecorder()
	handler.ServeHTTP(rr2, req2)
	if rr2.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429, got %d", rr2.Code)
	}

	// Check headers
	limitHeader := rr2.Header().Get("RateLimit-Limit")
	if limitHeader != "1;window=3600" {
		t.Errorf("expected RateLimit-Limit: 1;window=3600, got %s", limitHeader)
	}

	if rr2.Header().Get("X-RateLimit-Burst") != "" {
		t.Errorf("X-RateLimit-Burst should be empty")
	}
}

func TestCleanup(t *testing.T) {
	l := New(time.Second, WithCleanInterval(time.Millisecond*100), WithCleanCutoff(time.Millisecond*200))
	defer l.Close()

	l.Allow("user1")
	l.Allow("user2")

	l.lock.Lock()
	count := len(l.limiters)
	l.lock.Unlock()
	if count != 2 {
		t.Errorf("expected 2 limiters, got %d", count)
	}

	// Wait for cleanup
	time.Sleep(time.Millisecond * 500)

	l.lock.Lock()
	count = len(l.limiters)
	l.lock.Unlock()
	if count != 0 {
		t.Errorf("expected 0 limiters after cleanup, got %d", count)
	}
}
