package handler

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// rateLimiter is a process-local, fixed-window per-key request limiter used to
// throttle unauthenticated endpoints (login, refresh) against brute force. It
// is best-effort: counts live in memory and reset on restart.
type rateLimiter struct {
	mu     sync.Mutex
	counts map[string]*rateWindow
	limit  int
	window time.Duration
}

type rateWindow struct {
	count int
	reset time.Time
}

// newRateLimiter allows up to limit requests per key within each window.
func newRateLimiter(limit int, window time.Duration) *rateLimiter {
	return &rateLimiter{
		counts: make(map[string]*rateWindow),
		limit:  limit,
		window: window,
	}
}

// allow records a request for key and reports whether it is within the limit.
func (rl *rateLimiter) allow(key string) bool {
	now := time.Now()

	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Opportunistically prune expired entries so the map cannot grow without
	// bound under a flood of distinct client IPs.
	if len(rl.counts) > 10000 {
		for k, w := range rl.counts {
			if now.After(w.reset) {
				delete(rl.counts, k)
			}
		}
	}

	w, ok := rl.counts[key]
	if !ok || now.After(w.reset) {
		rl.counts[key] = &rateWindow{count: 1, reset: now.Add(rl.window)}
		return true
	}
	if w.count >= rl.limit {
		return false
	}
	w.count++
	return true
}

// Middleware throttles requests by client IP, returning 429 when over the limit.
func (rl *rateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !rl.allow(c.ClientIP()) {
			c.JSON(http.StatusTooManyRequests, errorBody{
				Code:    "rate_limited",
				Message: "too many requests, please try again later",
			})
			c.Abort()
			return
		}
		c.Next()
	}
}
