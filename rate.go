package rate

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// New returns a new Limiters instance.
// minInterval is the minimum interval between requests for a single key.
func New(minInterval time.Duration, options ...Option) *Limiters {

	l := &Limiters{
		limiters:      map[string]*limiter{},
		minInterval:   minInterval,
		burst:         1,
		cleanInterval: time.Minute,
		cleanCutoff:   time.Hour,
		stop:          make(chan struct{}),
	}

	for _, option := range options {
		option(l)
	}

	go l.clean()

	return l
}

// Limiters manages a collection of rate limiters, one per key.
type Limiters struct {
	limiters      map[string]*limiter
	lock          sync.Mutex
	minInterval   time.Duration
	burst         int
	cleanInterval time.Duration
	cleanCutoff   time.Duration
	bucketName    string
	stop          chan struct{}
}

// Close stops the background cleanup goroutine.
func (l *Limiters) Close() {
	close(l.stop)
}

type limiter struct {
	limiter *rate.Limiter
	updated time.Time
}

// GetBurst returns the burst size.
func (l *Limiters) GetBurst() int {
	return l.burst
}

// GetMinInterval returns the minimum interval between requests.
func (l *Limiters) GetMinInterval() time.Duration {
	return l.minInterval
}

// GetBucketName returns the bucket name used in headers.
func (l *Limiters) GetBucketName() string {
	return l.bucketName
}

// Allow is a shorthand for GetLimiter(key).Allow().
func (l *Limiters) Allow(key string) bool {
	return l.GetLimiter(key).Allow()
}

// Wait is a shorthand for GetLimiter(key).Wait(ctx).
func (l *Limiters) Wait(ctx context.Context, key string) error {
	return l.GetLimiter(key).Wait(ctx)
}

// Reserve is a shorthand for GetLimiter(key).Reserve().
func (l *Limiters) Reserve(key string) *rate.Reservation {
	return l.GetLimiter(key).Reserve()
}

// GetLimiter returns the rate.Limiter for the provided key.
// It creates a new limiter if one doesn't exist for the key.
func (l *Limiters) GetLimiter(key string) *rate.Limiter {

	l.lock.Lock()
	defer l.lock.Unlock()

	lim, exists := l.limiters[key]

	if !exists {

		lim = &limiter{
			limiter: rate.NewLimiter(rate.Every(l.minInterval), l.burst),
		}

		l.limiters[key] = lim
	}

	// Touch limiter
	lim.updated = time.Now()

	return lim.limiter
}

func (l *Limiters) clean() {

	ticker := time.NewTicker(l.cleanInterval)
	defer ticker.Stop()

	for {
		select {
		case <-l.stop:
			return
		case <-ticker.C:
			cutoff := time.Now().Add(l.cleanCutoff * -1)

			l.lock.Lock()
			for k, v := range l.limiters {
				if v.updated.Before(cutoff) {
					delete(l.limiters, k)
				}
			}
			l.lock.Unlock()
		}
	}
}

// Option is a function that configures a Limiters instance.
type Option func(l *Limiters)

// WithBurst sets the burst size for the limiters.
func WithBurst(burst int) Option {
	return func(l *Limiters) {
		l.burst = burst
	}
}

// WithCleanCutoff sets how long a limiter must be idle before it's removed.
func WithCleanCutoff(duration time.Duration) Option {
	return func(l *Limiters) {
		l.cleanCutoff = duration
	}
}

// WithCleanInterval sets how often the background cleanup goroutine runs.
func WithCleanInterval(duration time.Duration) Option {
	return func(l *Limiters) {
		l.cleanInterval = duration
	}
}

// WithBucketName sets the bucket name used in headers.
func WithBucketName(name string) Option {
	return func(l *Limiters) {
		l.bucketName = name
	}
}

// Middleware returns a middleware that rate limits requests based on the provided key function.
// If keyFn is nil, it defaults to using the remote address.
func (l *Limiters) Middleware(next http.Handler, keyFn func(r *http.Request) string) http.Handler {
	if keyFn == nil {
		keyFn = func(r *http.Request) string {
			return r.RemoteAddr
		}
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := keyFn(r)
		reservation := l.Reserve(key)

		if !reservation.OK() {
			SetRateLimitHeaders(w, l, reservation)
			http.Error(w, http.StatusText(http.StatusTooManyRequests), http.StatusTooManyRequests)
			return
		}

		// Wait if necessary (though Reserve normally handles this, but here we check OK() first)
		// If we wanted to block, we would use Wait().
		// Since we want to error with 429 if the rate limit is exceeded, we use Reserve and check OK.
		// However, Reserve(key).OK() is always true unless the limit is 0.
		// We actually want to check if the reservation requires waiting.
		if reservation.Delay() > 0 {
			SetRateLimitHeaders(w, l, reservation)
			http.Error(w, http.StatusText(http.StatusTooManyRequests), http.StatusTooManyRequests)
			reservation.Cancel()
			return
		}

		next.ServeHTTP(w, r)
	})
}

// SetRateLimitHeaders sets standard IETF draft rate limit headers on the response.
// See: https://datatracker.ietf.org/doc/draft-ietf-httpapi-ratelimit-headers/
func SetRateLimitHeaders(w http.ResponseWriter, limiters *Limiters, reservation *rate.Reservation) {

	// RateLimit-Limit: contains the rate limit ceiling and parameters.
	// Format: <limit>;window=<seconds>;policy="<name>"
	limit := limiters.GetBurst()
	window := limiters.GetMinInterval().Seconds()
	limitHeader := fmt.Sprintf("%d;window=%.0f", limit, window)

	if bucket := limiters.GetBucketName(); bucket != "" {
		limitHeader += fmt.Sprintf(";policy=\"%s\"", bucket)
	}
	w.Header().Set("RateLimit-Limit", limitHeader)

	// RateLimit-Remaining: number of requests left (approximate).
	remaining := "0"
	if reservation.Delay() == 0 {
		remaining = "1" // We don't have exact remaining, but if delay is 0, at least 1 is possible
	}
	w.Header().Set("RateLimit-Remaining", remaining)

	// RateLimit-Reset: seconds until the window resets or next request is allowed.
	w.Header().Set("RateLimit-Reset", fmt.Sprintf("%.0f", reservation.Delay().Seconds()))
}
