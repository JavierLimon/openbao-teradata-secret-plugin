package odbc

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestValidateUsername(t *testing.T) {
	tests := []struct {
		name      string
		username  string
		wantError bool
	}{
		{"valid username", "validuser", false},
		{"valid username with underscore", "valid_user", false},
		{"valid username with dollar", "valid$user", false},
		{"valid username with numbers", "user123", false},
		{"empty username", "", true},
		{"username 31 chars", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", true},
		{"username with semicolon (injection)", "user; DROP TABLE", true},
		{"username with double dash (injection)", "user--test", true},
		{"username with comment start", "user/*test", true},
		{"username with SELECT keyword", "SELECTuser", true},
		{"username with DROP keyword", "DROPuser", true},
		{"username with INSERT keyword", "INSERTuser", true},
		{"username with UPDATE keyword", "UPDATEuser", true},
		{"username with DELETE keyword", "DELETEuser", true},
		{"username with GRANT keyword", "GRANTuser", true},
		{"username with xp_ pattern", "xp_test", true},
		{"username with sp_ pattern", "sp_test", true},
		{"username with waitfor pattern", "waitfor_delay", true},
		{"username with invalid char space", "user test", true},
		{"username with invalid char quote", "user'name", true},
		{"username with invalid char dash", "user-name", true},
		{"username with invalid char at", "user@name", true},
		{"case insensitive SELECT", "selectuser", true},
		{"case insensitive DROP", "dropuser", true},
		{"lowercase injection", "user; drop user dbc", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateUsername(tt.username)
			if tt.wantError && err == nil {
				t.Errorf("ValidateUsername() expected error for %q, got nil", tt.username)
			} else if !tt.wantError && err != nil {
				t.Errorf("ValidateUsername() unexpected error for %q: %v", tt.username, err)
			}
		})
	}
}

func TestConnectionValidation(t *testing.T) {
	conn := &Connection{
		connected: false,
		db:        nil,
	}

	if err := conn.Validate(context.Background()); err != ErrNotConnected {
		t.Errorf("Validate() expected ErrNotConnected for unconnected connection, got: %v", err)
	}

	conn.connected = true
	conn.db = nil

	if err := conn.Validate(context.Background()); err != ErrNotConnected {
		t.Errorf("Validate() expected ErrNotConnected for nil db, got: %v", err)
	}
}

func TestConnectionLastValidated(t *testing.T) {
	conn := &Connection{}

	ts := conn.LastValidated()
	if !ts.IsZero() {
		t.Errorf("LastValidated() expected zero time for new connection, got: %v", ts)
	}
}

func TestConnectionIsHealthy(t *testing.T) {
	conn := &Connection{}

	if conn.IsHealthy() {
		t.Errorf("IsHealthy() expected false for new connection")
	}

	conn.connected = true
	conn.db = nil

	if conn.IsHealthy() {
		t.Errorf("IsHealthy() expected false when db is nil")
	}
}

func TestConnectionSetKeepAliveInterval(t *testing.T) {
	conn := &Connection{}

	conn.SetKeepAliveInterval(60 * time.Second)

	conn.mu.RLock()
	interval := conn.keepAliveInt
	conn.mu.RUnlock()

	if interval != 60*time.Second {
		t.Errorf("SetKeepAliveInterval() expected 60s, got: %v", interval)
	}
}

func TestConnectionKeepAliveLifecycle(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	conn := &Connection{
		connected: true,
	}

	conn.SetKeepAliveInterval(100 * time.Millisecond)
	conn.StartKeepAlive(ctx)

	time.Sleep(50 * time.Millisecond)
	conn.StopKeepAlive()

	select {
	case <-conn.keepAliveDone:
	case <-time.After(time.Second):
		t.Errorf("StopKeepAlive() should have closed keepAliveDone channel")
	}
}

func TestConnectionStopKeepAliveTwice(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	conn := &Connection{
		connected: true,
	}

	conn.SetKeepAliveInterval(100 * time.Millisecond)
	conn.StartKeepAlive(ctx)

	time.Sleep(50 * time.Millisecond)

	conn.StopKeepAlive()
	conn.StopKeepAlive()
}

func TestDefaultConnectConfig(t *testing.T) {
	cfg := DefaultConnectConfig()

	if cfg.MaxRetries != DefaultMaxRetries {
		t.Errorf("DefaultConnectConfig() MaxRetries = %d, want %d", cfg.MaxRetries, DefaultMaxRetries)
	}
	if cfg.RetryInterval != DefaultRetryInterval {
		t.Errorf("DefaultConnectConfig() RetryInterval = %v, want %v", cfg.RetryInterval, DefaultRetryInterval)
	}
	if cfg.MaxInterval != DefaultMaxRetryInterval {
		t.Errorf("DefaultConnectConfig() MaxInterval = %v, want %v", cfg.MaxInterval, DefaultMaxRetryInterval)
	}
	if cfg.Multiplier != DefaultRetryMultiplier {
		t.Errorf("DefaultConnectConfig() Multiplier = %v, want %v", cfg.Multiplier, DefaultRetryMultiplier)
	}
}

func TestIsRetryableError(t *testing.T) {
	tests := []struct {
		name      string
		errMsg    string
		wantRetry bool
	}{
		{"ECONNREFUSED", "dial tcp: connection refused (ECONNREFUSED)", true},
		{"ECONNRESET", "read tcp: connection reset by peer (ECONNRESET)", true},
		{"ETIMEDOUT", "dial tcp: operation timed out (ETIMEDOUT)", true},
		{"connection refused text", "odbc: connection refused", true},
		{"connection reset text", "odbc: connection reset", true},
		{"timeout text", "operation timeout", true},
		{"temporary failure", "temporary failure in name resolution", true},
		{"deadlock", "database deadlock detected", true},
		{"lock contention", "lock contention on resource", true},
		{"resource unavailable", "resource unavailable", true},
		{"table is busy", "table is busy", true},
		{"nil error", "", false},
		{"no match error", "some random error", false},
		{"invalid credentials", "invalid username or password", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var err error
			if tt.errMsg != "" {
				err = errors.New(tt.errMsg)
			}
			got := isRetryableError(err)
			if got != tt.wantRetry {
				t.Errorf("isRetryableError(%q) = %v, want %v", tt.errMsg, got, tt.wantRetry)
			}
		})
	}
}

func TestCalculateBackoff(t *testing.T) {
	cfg := &ConnectConfig{
		RetryInterval: 100 * time.Millisecond,
		MaxInterval:   5 * time.Second,
		Multiplier:    2.0,
	}

	backoff1 := calculateConnectBackoff(1, cfg)
	if backoff1 < 100*time.Millisecond || backoff1 > 130*time.Millisecond {
		t.Errorf("calculateConnectBackoff(1) = %v, expected ~100-130ms", backoff1)
	}

	backoff2 := calculateConnectBackoff(2, cfg)
	if backoff2 < 200*time.Millisecond || backoff2 > 260*time.Millisecond {
		t.Errorf("calculateConnectBackoff(2) = %v, expected ~200-260ms", backoff2)
	}

	backoff3 := calculateConnectBackoff(3, cfg)
	if backoff3 < 400*time.Millisecond || backoff3 > 520*time.Millisecond {
		t.Errorf("calculateConnectBackoff(3) = %v, expected ~400-520ms", backoff3)
	}
}

func TestConnectWithRetryInvalidConnection(t *testing.T) {
	cfg := &ConnectConfig{
		MaxRetries:    2,
		RetryInterval: 50 * time.Millisecond,
		MaxInterval:   100 * time.Millisecond,
		Multiplier:    2.0,
	}

	conn, err := ConnectWithRetry("invalid-connection-string", cfg)
	if err == nil {
		t.Errorf("ConnectWithRetry() expected error for invalid connection, got nil")
		if conn != nil {
			conn.Close()
		}
	}
}

func TestConnectWithRetryConfigDefaults(t *testing.T) {
	conn, err := ConnectWithRetry("invalid-connection-string", nil)
	if err == nil {
		t.Errorf("ConnectWithRetry() expected error for invalid connection, got nil")
		if conn != nil {
			conn.Close()
		}
	}
}
