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

func ErrorMiddleware(limiters *Limiters, handler http.HandlerFunc) func(http.Handler) http.Handler {

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			reservation := limiters.GetLimiter(r.RemoteAddr).Reserve()
			if reservation.Delay() > 0 {

				SetRateLimitHeaders(w, limiters, reservation)
				handler(w, r)
				reservation.Cancel()
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func BlockMiddleware(limiters *Limiters, handler http.HandlerFunc, key func(*http.Request) string) func(http.Handler) http.Handler {

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			err := limiters.GetLimiter(key(r)).Wait(r.Context())
			if err != nil {
				handler(w, r)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
