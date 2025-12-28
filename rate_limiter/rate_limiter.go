package rate_limiter

import (
	"net/http"
	"time"
)

type RateLimiter struct {
	tokens chan struct{}
}

func NewRateLimiter(rps int, burst int) *RateLimiter {
	rl := &RateLimiter{
		tokens: make(chan struct{}, burst),
	}

	for i := 0; i < burst; i++ {
		rl.tokens <- struct{}{}
	}

	go func() {
		ticker := time.NewTicker(time.Second / time.Duration(rps))
		defer ticker.Stop()
		for range ticker.C {
			select {
			case rl.tokens <- struct{}{}:
			default:
				// bucket full
			}
		}
	}()

	return rl
}

func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-rl.tokens:
			next.ServeHTTP(w, r)
		default:
			http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
		}
	})
}
