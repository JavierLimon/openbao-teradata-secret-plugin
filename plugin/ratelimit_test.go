package teradata

import (
	"context"
	"testing"
	"time"

	"github.com/openbao/openbao/sdk/v2/logical"
)

func TestRateLimiter_Allow(t *testing.T) {
	cfg := RateLimitConfig{
		RequestsPerSecond: 2,
		BurstSize:         3,
		CleanupInterval:   time.Minute,
	}

	rl := NewRateLimiter(cfg)

	for i := 0; i < 3; i++ {
		if !rl.Allow("test-ip") {
			t.Errorf("Request %d should be allowed", i+1)
		}
	}

	if rl.Allow("test-ip") {
		t.Error("4th request should be denied")
	}
}

func TestRateLimiter_DifferentIPs(t *testing.T) {
	cfg := RateLimitConfig{
		RequestsPerSecond: 1,
		BurstSize:         1,
		CleanupInterval:   time.Minute,
	}

	rl := NewRateLimiter(cfg)

	if !rl.Allow("ip-1") {
		t.Error("ip-1 first request should be allowed")
	}
	if !rl.Allow("ip-2") {
		t.Error("ip-2 first request should be allowed")
	}
	if rl.Allow("ip-1") {
		t.Error("ip-1 second request should be denied")
	}
	if rl.Allow("ip-2") {
		t.Error("ip-2 second request should be denied")
	}
}

func TestRateLimiter_Refill(t *testing.T) {
	cfg := RateLimitConfig{
		RequestsPerSecond: 1,
		BurstSize:         1,
		CleanupInterval:   time.Minute,
	}

	rl := NewRateLimiter(cfg)

	if !rl.Allow("test-ip") {
		t.Error("First request should be allowed")
	}

	if rl.Allow("test-ip") {
		t.Error("Second immediate request should be denied")
	}

	time.Sleep(time.Second)

	if !rl.Allow("test-ip") {
		t.Error("Request after 1 second should be allowed")
	}
}

func TestRateLimiter_ConfigValidation(t *testing.T) {
	testCases := []struct {
		name    string
		cfg     RateLimitConfig
		wantErr bool
	}{
		{
			name: "valid config",
			cfg: RateLimitConfig{
				RequestsPerSecond: 100,
				BurstSize:         50,
				CleanupInterval:   5 * time.Minute,
			},
			wantErr: false,
		},
		{
			name: "zero values use defaults",
			cfg: RateLimitConfig{
				RequestsPerSecond: 0,
				BurstSize:         0,
				CleanupInterval:   0,
			},
			wantErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			rl := NewRateLimiter(tc.cfg)
			if rl == nil {
				t.Error("Expected non-nil rate limiter")
			}
		})
	}
}

func TestRateLimitError(t *testing.T) {
	err := &RateLimitError{
		Key:     "192.168.1.1",
		RetryIn: time.Second,
	}

	if err.Error() != "rate limit exceeded" {
		t.Errorf("Unexpected error message: %s", err.Error())
	}
}

func TestGetClientIP(t *testing.T) {
	testCases := []struct {
		name     string
		req      *logical.Request
		expected string
	}{
		{
			name:     "nil request",
			req:      nil,
			expected: "unknown",
		},
		{
			name:     "empty connection",
			req:      &logical.Request{},
			expected: "unknown",
		},
		{
			name: "valid IP with port",
			req: &logical.Request{
				Connection: &logical.Connection{
					RemoteAddr: "192.168.1.1:12345",
				},
			},
			expected: "192.168.1.1",
		},
		{
			name: "IPv6 address",
			req: &logical.Request{
				Connection: &logical.Connection{
					RemoteAddr: "[::1]:12345",
				},
			},
			expected: "::1",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ip := getClientIP(tc.req)
			if ip != tc.expected {
				t.Errorf("Expected %s, got %s", tc.expected, ip)
			}
		})
	}
}

func TestRateLimiterMiddleware_IsExcluded(t *testing.T) {
	cfg := RateLimitConfig{
		RequestsPerSecond: 100,
		BurstSize:         50,
		CleanupInterval:   5 * time.Minute,
	}

	backend := &Backend{}
	rlm := NewRateLimiterMiddleware(backend, cfg, true)

	testCases := []struct {
		path     string
		excluded bool
	}{
		{"health", true},
		{"version", true},
		{"api-version", true},
		{"pool-stats", true},
		{"creds/myrole", false},
		{"config", false},
		{"roles", false},
	}

	for _, tc := range testCases {
		t.Run(tc.path, func(t *testing.T) {
			if rlm.isExcluded(tc.path) != tc.excluded {
				t.Errorf("Path %s: expected excluded=%v, got %v", tc.path, tc.excluded, rlm.isExcluded(tc.path))
			}
		})
	}
}

func TestRateLimiterMiddleware_RateLimit(t *testing.T) {
	cfg := RateLimitConfig{
		RequestsPerSecond: 1,
		BurstSize:         2,
		CleanupInterval:   time.Minute,
	}

	backend := &Backend{}
	rlm := NewRateLimiterMiddleware(backend, cfg, true)

	ctx := context.Background()

	for i := 0; i < 2; i++ {
		req := &logical.Request{
			Path: "creds/test",
			Connection: &logical.Connection{
				RemoteAddr: "10.0.0.1:12345",
			},
		}
		err := rlm.RateLimit(ctx, req)
		if err != nil {
			t.Errorf("Request %d should be allowed, got error: %v", i+1, err)
		}
	}

	req := &logical.Request{
		Path: "creds/test",
		Connection: &logical.Connection{
			RemoteAddr: "10.0.0.1:12345",
		},
	}
	err := rlm.RateLimit(ctx, req)
	if err == nil {
		t.Error("Third request should be rate limited")
	}

	_, ok := err.(*RateLimitError)
	if !ok {
		t.Error("Error should be RateLimitError")
	}
}

func TestRateLimiterMiddleware_ExcludedPathsNotLimited(t *testing.T) {
	cfg := RateLimitConfig{
		RequestsPerSecond: 1,
		BurstSize:         1,
		CleanupInterval:   time.Minute,
	}

	backend := &Backend{}
	rlm := NewRateLimiterMiddleware(backend, cfg, true)

	ctx := context.Background()
	excludedPaths := []string{"health", "version", "api-version", "pool-stats"}

	for _, path := range excludedPaths {
		for i := 0; i < 10; i++ {
			req := &logical.Request{
				Path: path,
				Connection: &logical.Connection{
					RemoteAddr: "10.0.0.1:12345",
				},
			}
			err := rlm.RateLimit(ctx, req)
			if err != nil {
				t.Errorf("Excluded path %s should not be rate limited, got error: %v", path, err)
			}
		}
	}
}

func TestRateLimiterMiddleware_Disabled(t *testing.T) {
	cfg := RateLimitConfig{
		RequestsPerSecond: 1,
		BurstSize:         1,
		CleanupInterval:   time.Minute,
	}

	backend := &Backend{}
	rlm := NewRateLimiterMiddleware(backend, cfg, false)

	ctx := context.Background()

	for i := 0; i < 100; i++ {
		req := &logical.Request{
			Path: "creds/test",
			Connection: &logical.Connection{
				RemoteAddr: "10.0.0.1:12345",
			},
		}
		err := rlm.RateLimit(ctx, req)
		if err != nil {
			t.Errorf("When disabled, request %d should be allowed, got error: %v", i+1, err)
		}
	}
}

func TestBackend_RateLimitIntegration(t *testing.T) {
	b := NewBackend()

	cfg := &logical.BackendConfig{
		StorageView: &logical.InmemStorage{},
	}
	ctx := context.Background()

	err := b.Setup(ctx, cfg)
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	if !b.IsRateLimiterEnabled() {
		t.Error("Rate limiter should be enabled by default")
	}

	cfg2 := b.GetRateLimitConfig()
	if cfg2.RequestsPerSecond != 100 {
		t.Errorf("Expected 100 requests per second, got %f", cfg2.RequestsPerSecond)
	}

	b.SetRateLimiterEnabled(false)
	if b.IsRateLimiterEnabled() {
		t.Error("Rate limiter should be disabled")
	}

	b.SetRateLimiterEnabled(true)
	if !b.IsRateLimiterEnabled() {
		t.Error("Rate limiter should be enabled")
	}
}
