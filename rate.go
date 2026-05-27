package rate

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

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

func (l *Limiters) GetBurst() int {
	return l.burst
}

func (l *Limiters) GetMinInterval() time.Duration {
	return l.minInterval
}

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

type Option func(l *Limiters)

func WithBurst(burst int) Option {
	return func(l *Limiters) {
		l.burst = burst
	}
}

func WithCleanCutoff(duration time.Duration) Option {
	return func(l *Limiters) {
		l.cleanCutoff = duration
	}
}

func WithCleanInterval(duration time.Duration) Option {
	return func(l *Limiters) {
		l.cleanInterval = duration
	}
}

func WithBucketName(name string) Option {
	return func(l *Limiters) {
		l.bucketName = name
	}
}

func SetRateLimitHeaders(w http.ResponseWriter, limiters *Limiters, reservation *rate.Reservation) {

	w.Header().Set("X-RateLimit-Every", limiters.GetMinInterval().String())
	w.Header().Set("X-RateLimit-Burst", fmt.Sprint(limiters.GetBurst()))
	w.Header().Set("X-RateLimit-Wait", reservation.Delay().String())

	if bucket := limiters.GetBucketName(); bucket != "" {
		w.Header().Set("X-RateLimit-Bucket", bucket)
	}
}
