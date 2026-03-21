package teradata

import (
	"context"
	"net"
	"sync"
	"time"

	"github.com/openbao/openbao/sdk/v2/logical"
	"golang.org/x/time/rate"
)

type RateLimiter struct {
	mu       sync.RWMutex
	limiters map[string]*rate.Limiter
	rate     rate.Limit
	burst    int
	cleanup  time.Duration
}

type RateLimitConfig struct {
	RequestsPerSecond float64
	BurstSize         int
	CleanupInterval   time.Duration
}

func NewRateLimiter(cfg RateLimitConfig) *RateLimiter {
	if cfg.RequestsPerSecond <= 0 {
		cfg.RequestsPerSecond = 100
	}
	if cfg.BurstSize <= 0 {
		cfg.BurstSize = 50
	}
	if cfg.CleanupInterval <= 0 {
		cfg.CleanupInterval = 5 * time.Minute
	}

	rl := &RateLimiter{
		limiters: make(map[string]*rate.Limiter),
		rate:     rate.Limit(cfg.RequestsPerSecond),
		burst:    cfg.BurstSize,
		cleanup:  cfg.CleanupInterval,
	}

	go rl.cleanupLoop()
	return rl
}

func (rl *RateLimiter) getLimiter(key string) *rate.Limiter {
	rl.mu.RLock()
	limiter, exists := rl.limiters[key]
	rl.mu.RUnlock()

	if exists {
		return limiter
	}

	rl.mu.Lock()
	defer rl.mu.Unlock()

	if limiter, exists = rl.limiters[key]; exists {
		return limiter
	}

	limiter = rate.NewLimiter(rl.rate, rl.burst)
	rl.limiters[key] = limiter
	return limiter
}

func (rl *RateLimiter) Allow(key string) bool {
	limiter := rl.getLimiter(key)
	return limiter.Allow()
}

func (rl *RateLimiter) cleanupLoop() {
	ticker := time.NewTicker(rl.cleanup)
	defer ticker.Stop()

	for range ticker.C {
		rl.cleanupExpired()
	}
}

func (rl *RateLimiter) cleanupExpired() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	if len(rl.limiters) > 1000 {
		rl.limiters = make(map[string]*rate.Limiter)
	}
}

func (rl *RateLimiter) GetLimit(key string) (rate.Limit, int) {
	limiter := rl.getLimiter(key)
	return limiter.Limit(), limiter.Burst()
}

type RateLimitError struct {
	Key       string
	RetryIn   time.Duration
	Rate      rate.Limit
	Burst     int
	Remaining int
}

func (e *RateLimitError) Error() string {
	return "rate limit exceeded"
}

func getClientIP(req *logical.Request) string {
	if req == nil {
		return "unknown"
	}

	if req.Connection != nil && req.Connection.RemoteAddr != "" {
		addr := req.Connection.RemoteAddr
		ip, _, err := net.SplitHostPort(addr)
		if err == nil {
			return ip
		}
		return addr
	}

	return "unknown"
}

type RateLimiterMiddleware struct {
	backend      *Backend
	limiter      *RateLimiter
	enabled      bool
	excludePaths map[string]bool
}

func NewRateLimiterMiddleware(backend *Backend, cfg RateLimitConfig, enabled bool) *RateLimiterMiddleware {
	rlm := &RateLimiterMiddleware{
		backend:      backend,
		limiter:      NewRateLimiter(cfg),
		enabled:      enabled,
		excludePaths: make(map[string]bool),
	}

	rlm.excludePaths["health"] = true
	rlm.excludePaths["version"] = true
	rlm.excludePaths["api-version"] = true
	rlm.excludePaths["pool-stats"] = true

	return rlm
}

func (rlm *RateLimiterMiddleware) isExcluded(path string) bool {
	for prefix, _ := range rlm.excludePaths {
		if len(path) >= len(prefix) && path[:len(prefix)] == prefix {
			return true
		}
	}
	return false
}

func (rlm *RateLimiterMiddleware) RateLimit(ctx context.Context, req *logical.Request) error {
	if !rlm.enabled {
		return nil
	}

	path := req.Path
	if rlm.isExcluded(path) {
		return nil
	}

	ip := getClientIP(req)
	if !rlm.limiter.Allow(ip) {
		return &RateLimitError{
			Key:     ip,
			RetryIn: time.Second,
			Rate:    rlm.limiter.rate,
			Burst:   rlm.limiter.burst,
		}
	}

	return nil
}

func (rlm *RateLimiterMiddleware) SetEnabled(enabled bool) {
	rlm.enabled = enabled
}

func (rlm *RateLimiterMiddleware) IsEnabled() bool {
	return rlm.enabled
}

func (rlm *RateLimiterMiddleware) GetConfig() RateLimitConfig {
	return RateLimitConfig{
		RequestsPerSecond: float64(rlm.limiter.rate),
		BurstSize:         rlm.limiter.burst,
		CleanupInterval:   rlm.limiter.cleanup,
	}
}
