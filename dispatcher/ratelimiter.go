package dispatcher

import (
	"context"
	"sync"

	"github.com/iotexproject/go-pkgs/cache"
	"golang.org/x/time/rate"
)

// RateLimiter is a struct that manages a threadsafe map of rate limiters.
type RateLimiter struct {
	mu       sync.RWMutex
	limiters cache.LRUCache
	r        rate.Limit
	b        int
}

// NewRateLimiter creates a new RateLimiter.
func NewRateLimiter(s int, r rate.Limit, b int) *RateLimiter {
	return &RateLimiter{
		limiters: cache.NewThreadSafeLruCache(s),
		r:        r,
		b:        b,
	}
}

// getLimiter retrieves the rate limiter for the given key, creating a new one if it doesn't exist.
func (rl *RateLimiter) getLimiter(key string) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	limiter, exists := rl.limiters.Get(key)
	if !exists {
		limiter = rate.NewLimiter(rl.r, rl.b)
		rl.limiters.Add(key, limiter)
	}
	return limiter.(*rate.Limiter)
}

// Remainings returns the number of remaining tokens for the given key.
func (rl *RateLimiter) Remainings(key string) int {
	return int(rl.getLimiter(key).Tokens())
}

// Wait waits for 1 token to become available for the given key, up to the given duration.
func (rl *RateLimiter) Wait(key string) {
	rl.getLimiter(key).Wait(context.Background())
}
