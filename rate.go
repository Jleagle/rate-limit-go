package rate

import (
	"sync"
	"time"

	"golang.org/x/time/rate"
)

func New(per time.Duration, options ...Option) *limiters {

	l := &limiters{
		limiters:      map[string]*limiter{},
		limit:         rate.Every(per),
		burst:         1,
		cleanInterval: time.Minute,
		cleanCutoff:   time.Hour,
	}

	for _, option := range options {
		option(l)
	}

	go l.clean()

	return l
}

type limiters struct {
	limiters      map[string]*limiter
	lock          sync.Mutex
	limit         rate.Limit
	burst         int
	cleanInterval time.Duration
	cleanCutoff   time.Duration
}

type limiter struct {
	limiter *rate.Limiter
	updated time.Time
}

func (l *limiters) GetLimiter(key string) *rate.Limiter {

	l.lock.Lock()
	defer l.lock.Unlock()

	lim, exists := l.limiters[key]

	if !exists {

		lim = &limiter{
			limiter: rate.NewLimiter(l.limit, l.burst),
		}

		l.limiters[key] = lim
	}

	// Touch limiter
	lim.updated = time.Now()

	return lim.limiter
}

func (l *limiters) clean() {

	for {
		time.Sleep(l.cleanInterval)

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

type Option func(l *limiters)

func WithBurst(burst int) Option {
	return func(l *limiters) {
		l.burst = burst
	}
}

func WithCleanCutoff(duration time.Duration) Option {
	return func(l *limiters) {
		l.cleanCutoff = duration
	}
}

func WithCleanInterval(duration time.Duration) Option {
	return func(l *limiters) {
		l.cleanInterval = duration
	}
}
