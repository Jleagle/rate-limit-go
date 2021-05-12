package rate

import (
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
