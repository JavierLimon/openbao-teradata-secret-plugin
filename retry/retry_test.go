package retry

import (
	"context"
	"errors"
	"testing"
	"time"
)

type mockError struct {
	msg       string
	transient bool
}

func (e *mockError) Error() string {
	return e.msg
}

func (e *mockError) IsTransient() bool {
	return e.transient
}

func TestIsTransientError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "timeout error",
			err:      errors.New("connection timeout"),
			expected: true,
		},
		{
			name:     "connection refused",
			err:      errors.New("connection refused"),
			expected: true,
		},
		{
			name:     "connection reset",
			err:      errors.New("connection reset by peer"),
			expected: true,
		},
		{
			name:     "deadlock",
			err:      errors.New("deadlock detected"),
			expected: true,
		},
		{
			name:     "lock contention",
			err:      errors.New("lock contention"),
			expected: true,
		},
		{
			name:     "permanent error",
			err:      errors.New("invalid username"),
			expected: false,
		},
		{
			name:     "transient error via interface",
			err:      &mockError{msg: "temp failure", transient: true},
			expected: true,
		},
		{
			name:     "non-transient error via interface",
			err:      &mockError{msg: "perm failure", transient: false},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsTransientError(tt.err)
			if result != tt.expected {
				t.Errorf("IsTransientError(%v) = %v, want %v", tt.err, result, tt.expected)
			}
		})
	}
}

func TestIsRetryableError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "timeout error",
			err:      errors.New("timeout"),
			expected: true,
		},
		{
			name:     "connection refused",
			err:      errors.New("connection refused"),
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsRetryableError(tt.err)
			if result != tt.expected {
				t.Errorf("IsRetryableError(%v) = %v, want %v", tt.err, result, tt.expected)
			}
		})
	}
}

func TestDo_Success(t *testing.T) {
	cfg := &Config{
		MaxAttempts:     3,
		InitialInterval: 10 * time.Millisecond,
		MaxInterval:     50 * time.Millisecond,
		Multiplier:      2.0,
	}

	callCount := 0
	err := Do(context.Background(), cfg, func() error {
		callCount++
		return nil
	})

	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
	if callCount != 1 {
		t.Errorf("expected 1 call, got %d", callCount)
	}
}

func TestDo_TransientError_Retries(t *testing.T) {
	cfg := &Config{
		MaxAttempts:     3,
		InitialInterval: 10 * time.Millisecond,
		MaxInterval:     50 * time.Millisecond,
		Multiplier:      2.0,
	}

	callCount := 0
	err := Do(context.Background(), cfg, func() error {
		callCount++
		if callCount < 3 {
			return errors.New("connection timeout")
		}
		return nil
	})

	if err != nil {
		t.Errorf("expected nil error after retries, got %v", err)
	}
	if callCount != 3 {
		t.Errorf("expected 3 calls, got %d", callCount)
	}
}

func TestDo_PermanentError_NoRetry(t *testing.T) {
	cfg := &Config{
		MaxAttempts:     3,
		InitialInterval: 10 * time.Millisecond,
		MaxInterval:     50 * time.Millisecond,
		Multiplier:      2.0,
	}

	callCount := 0
	err := Do(context.Background(), cfg, func() error {
		callCount++
		return errors.New("invalid syntax")
	})

	if err == nil {
		t.Error("expected error, got nil")
	}
	if callCount != 1 {
		t.Errorf("expected 1 call for permanent error, got %d", callCount)
	}
}

func TestDo_MaxAttemptsExceeded(t *testing.T) {
	cfg := &Config{
		MaxAttempts:     2,
		InitialInterval: 10 * time.Millisecond,
		MaxInterval:     50 * time.Millisecond,
		Multiplier:      2.0,
	}

	callCount := 0
	err := Do(context.Background(), cfg, func() error {
		callCount++
		return errors.New("connection timeout")
	})

	if err == nil {
		t.Error("expected error after max attempts, got nil")
	}
	if callCount != 2 {
		t.Errorf("expected 2 calls (max attempts), got %d", callCount)
	}
}

func TestDo_ContextCancelled(t *testing.T) {
	cfg := &Config{
		MaxAttempts:     5,
		InitialInterval: 50 * time.Millisecond,
		MaxInterval:     100 * time.Millisecond,
		Multiplier:      2.0,
	}

	ctx, cancel := context.WithCancel(context.Background())
	callCount := 0

	go func() {
		time.Sleep(25 * time.Millisecond)
		cancel()
	}()

	err := Do(ctx, cfg, func() error {
		callCount++
		return errors.New("connection timeout")
	})

	if err != context.Canceled {
		t.Errorf("expected context.Canceled, got %v", err)
	}
	if callCount != 1 {
		t.Errorf("expected 1 call before cancellation, got %d", callCount)
	}
}

func TestDo_OnRetryCallback(t *testing.T) {
	cfg := &Config{
		MaxAttempts:     3,
		InitialInterval: 10 * time.Millisecond,
		MaxInterval:     50 * time.Millisecond,
		Multiplier:      2.0,
	}

	callCount := 0
	retryCount := 0
	expectedErr := errors.New("connection timeout")

	cfg.OnRetry = func(attempt int, err error) {
		retryCount++
		if err != expectedErr {
			t.Errorf("expected error %v, got %v", expectedErr, err)
		}
	}

	err := Do(context.Background(), cfg, func() error {
		callCount++
		return expectedErr
	})

	if err == nil {
		t.Error("expected error after retries")
	}
	if callCount != 3 {
		t.Errorf("expected 3 calls, got %d", callCount)
	}
	if retryCount != 2 {
		t.Errorf("expected 2 retry callbacks, got %d", retryCount)
	}
}

func TestDoWithResult_Success(t *testing.T) {
	cfg := &Config{
		MaxAttempts:     3,
		InitialInterval: 10 * time.Millisecond,
		MaxInterval:     50 * time.Millisecond,
		Multiplier:      2.0,
	}

	result, err := DoWithResult(context.Background(), cfg, func() (interface{}, error) {
		return "success", nil
	})

	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
	if result != "success" {
		t.Errorf("expected 'success', got %v", result)
	}
}

func TestDoWithResult_TransientError_Retries(t *testing.T) {
	cfg := &Config{
		MaxAttempts:     3,
		InitialInterval: 10 * time.Millisecond,
		MaxInterval:     50 * time.Millisecond,
		Multiplier:      2.0,
	}

	callCount := 0
	result, err := DoWithResult(context.Background(), cfg, func() (interface{}, error) {
		callCount++
		if callCount < 3 {
			return nil, errors.New("connection timeout")
		}
		return "success", nil
	})

	if err != nil {
		t.Errorf("expected nil error after retries, got %v", err)
	}
	if result != "success" {
		t.Errorf("expected 'success', got %v", result)
	}
	if callCount != 3 {
		t.Errorf("expected 3 calls, got %d", callCount)
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.MaxAttempts != DefaultMaxAttempts {
		t.Errorf("expected MaxAttempts %d, got %d", DefaultMaxAttempts, cfg.MaxAttempts)
	}
	if cfg.InitialInterval != DefaultInitialInterval {
		t.Errorf("expected InitialInterval %v, got %v", DefaultInitialInterval, cfg.InitialInterval)
	}
	if cfg.MaxInterval != DefaultMaxInterval {
		t.Errorf("expected MaxInterval %v, got %v", DefaultMaxInterval, cfg.MaxInterval)
	}
	if cfg.Multiplier != DefaultMultiplier {
		t.Errorf("expected Multiplier %f, got %f", DefaultMultiplier, cfg.Multiplier)
	}
}

func TestCalculateBackoff(t *testing.T) {
	cfg := &Config{
		MaxAttempts:     5,
		InitialInterval: 100 * time.Millisecond,
		MaxInterval:     1 * time.Second,
		Multiplier:      2.0,
	}

	backoff1 := calculateBackoff(1, cfg)
	if backoff1 < 100*time.Millisecond || backoff1 > 130*time.Millisecond {
		t.Errorf("expected backoff around 100-130ms for attempt 1, got %v", backoff1)
	}

	backoff2 := calculateBackoff(2, cfg)
	if backoff2 < 200*time.Millisecond || backoff2 > 260*time.Millisecond {
		t.Errorf("expected backoff around 200-260ms for attempt 2, got %v", backoff2)
	}

	backoff3 := calculateBackoff(3, cfg)
	if backoff3 < 400*time.Millisecond || backoff3 > 520*time.Millisecond {
		t.Errorf("expected backoff around 400-520ms for attempt 3, got %v", backoff3)
	}
}

func TestCalculateBackoff_MaxIntervalCap(t *testing.T) {
	cfg := &Config{
		MaxAttempts:     10,
		InitialInterval: 100 * time.Millisecond,
		MaxInterval:     500 * time.Millisecond,
		Multiplier:      2.0,
	}

	for attempt := 1; attempt <= 10; attempt++ {
		backoff := calculateBackoff(attempt, cfg)
		if backoff > 650*time.Millisecond {
			t.Errorf("backoff for attempt %d exceeded max: %v", attempt, backoff)
		}
	}
}

func TestUntilSuccess(t *testing.T) {
	callCount := 0
	err := UntilSuccess(context.Background(), 3, func() error {
		callCount++
		if callCount < 3 {
			return errors.New("connection timeout")
		}
		return nil
	})

	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
	if callCount != 3 {
		t.Errorf("expected 3 calls, got %d", callCount)
	}
}

func TestNewTransientError(t *testing.T) {
	originalErr := errors.New("original error")
	transientErr := NewTransientError(originalErr)

	if transientErr.Error() != "original error" {
		t.Errorf("expected 'original error', got %s", transientErr.Error())
	}
	if !transientErr.IsTransient() {
		t.Error("expected IsTransient() to return true")
	}
	if errors.Unwrap(transientErr) != originalErr {
		t.Errorf("expected unwrap to return original error")
	}
}
