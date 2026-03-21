package odbc

import (
	"context"
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

	if err := conn.Validate(); err != ErrNotConnected {
		t.Errorf("Validate() expected ErrNotConnected for unconnected connection, got: %v", err)
	}

	conn.connected = true
	conn.db = nil

	if err := conn.Validate(); err != ErrNotConnected {
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
