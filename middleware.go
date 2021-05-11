package rate

import (
	"fmt"
	"net/http"

	"golang.org/x/time/rate"
)

func SetRateLimitHeaders(w http.ResponseWriter, limiters *Limiters, reservation *rate.Reservation) {

	w.Header().Set("X-RateLimit-Every", limiters.GetMinInterval().String())
	w.Header().Set("X-RateLimit-Burst", fmt.Sprint(limiters.GetBurst()))
	w.Header().Set("X-RateLimit-Wait", reservation.Delay().String())
	w.Header().Set("X-RateLimit-Bucket", "global")
}

func ErrorMiddleware(limiters func(*http.Request) *Limiters, errorHandler http.HandlerFunc) func(http.Handler) http.Handler {

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			limiters2 := limiters(r)

			reservation := limiters2.GetLimiter(r.RemoteAddr).Reserve()
			if reservation.Delay() > 0 {

				SetRateLimitHeaders(w, limiters2, reservation)
				errorHandler(w, r)
				reservation.Cancel()
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func BlockMiddleware(limiters func(*http.Request) *Limiters, key func(*http.Request) string, errorHandler func(http.ResponseWriter, *http.Request, error)) func(http.Handler) http.Handler {

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			err := limiters(r).GetLimiter(key(r)).Wait(r.Context())
			if err != nil {
				errorHandler(w, r, err)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
