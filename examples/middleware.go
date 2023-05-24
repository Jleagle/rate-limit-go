package examples

import (
	"net/http"
	"time"

	"github.com/Jleagle/rate-limit-go"
)

var limiters = rate.New(time.Second)

func Error(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		reservation := limiters.GetLimiter(r.RemoteAddr).Reserve()
		if !reservation.OK() {
			rate.SetRateLimitHeaders(w, limiters, reservation)
			http.Error(w, http.StatusText(http.StatusTooManyRequests), http.StatusTooManyRequests)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func Block(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		err := limiters.GetLimiter(r.RemoteAddr).Wait(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		next.ServeHTTP(w, r)
	})
}
