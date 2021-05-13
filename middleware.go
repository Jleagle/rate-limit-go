package rate

import (
	"fmt"
	"net/http"
)

func ErrorMiddlewareFunc(limiters func(*http.Request) *Limiters, key func(*http.Request) string, errorHandler http.Handler) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return errorMiddleware(limiters, key, errorHandler, next)
	}
}

func ErrorMiddleware(limiters func(*http.Request) *Limiters, key func(*http.Request) string, errorHandler http.Handler) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return errorMiddleware(limiters, key, errorHandler, next)
	}
}

func errorMiddleware(limiters func(*http.Request) *Limiters, key func(*http.Request) string, errorHandler http.Handler, next http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		limiters2 := limiters(r)

		reservation := limiters2.GetLimiter(key(r)).Reserve()
		if reservation.Delay() > 0 {

			w.Header().Set("X-RateLimit-Every", limiters2.GetMinInterval().String())
			w.Header().Set("X-RateLimit-Burst", fmt.Sprint(limiters2.GetBurst()))
			w.Header().Set("X-RateLimit-Wait", reservation.Delay().String())
			w.Header().Set("X-RateLimit-Bucket", limiters2.GetBucketName())

			errorHandler.ServeHTTP(w, r)
			reservation.Cancel()
			return
		}

		next.ServeHTTP(w, r)
	}
}

func BlockMiddlewareFunc(limiters func(*http.Request) *Limiters, key func(*http.Request) string, errorHandler func(http.ResponseWriter, *http.Request, error)) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return blockMiddleware(limiters, key, errorHandler, next)
	}
}

func BlockMiddleware(limiters func(*http.Request) *Limiters, key func(*http.Request) string, errorHandler func(http.ResponseWriter, *http.Request, error)) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return blockMiddleware(limiters, key, errorHandler, next)
	}
}

func blockMiddleware(limiters func(*http.Request) *Limiters, key func(*http.Request) string, errorHandler func(http.ResponseWriter, *http.Request, error), next http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		err := limiters(r).GetLimiter(key(r)).Wait(r.Context())
		if err != nil {
			errorHandler(w, r, err)
			return
		}

		next.ServeHTTP(w, r)
	}
}
