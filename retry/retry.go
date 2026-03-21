package retry

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"net/url"
	"strings"
	"time"

	"github.com/JavierLimon/openbao-teradata-secret-plugin/odbc"
)

const (
	DefaultMaxAttempts     = 3
	DefaultInitialInterval = 100 * time.Millisecond
	DefaultMaxInterval     = 5 * time.Second
	DefaultMultiplier      = 2.0
)

type Config struct {
	MaxAttempts     int
	InitialInterval time.Duration
	MaxInterval     time.Duration
	Multiplier      float64
	OnRetry         func(attempt int, err error)
}

func DefaultConfig() *Config {
	return &Config{
		MaxAttempts:     DefaultMaxAttempts,
		InitialInterval: DefaultInitialInterval,
		MaxInterval:     DefaultMaxInterval,
		Multiplier:      DefaultMultiplier,
	}
}

type TransientError interface {
	error
	IsTransient() bool
}

type transientError struct {
	err       error
	transient bool
}

func (e *transientError) Error() string {
	return e.err.Error()
}

func (e *transientError) Unwrap() error {
	return e.err
}

func (e *transientError) IsTransient() bool {
	return e.transient
}

func NewTransientError(err error) TransientError {
	return &transientError{err: err, transient: true}
}

func IsTransientError(err error) bool {
	if err == nil {
		return false
	}

	if te, ok := err.(TransientError); ok {
		return te.IsTransient()
	}

	if osErr, ok := err.(*url.Error); ok {
		return osErr.Temporary() || osErr.Timeout()
	}

	if strings.Contains(err.Error(), "ECONNREFUSED") ||
		strings.Contains(err.Error(), "ECONNRESET") ||
		strings.Contains(err.Error(), "ETIMEDOUT") ||
		strings.Contains(err.Error(), "ENETUNREACH") ||
		strings.Contains(err.Error(), "EHOSTUNREACH") ||
		strings.Contains(err.Error(), "connection refused") ||
		strings.Contains(err.Error(), "connection reset") ||
		strings.Contains(err.Error(), "timeout") ||
		strings.Contains(err.Error(), "temporary failure") ||
		strings.Contains(err.Error(), "server declined") ||
		strings.Contains(err.Error(), "unable to connect") ||
		strings.Contains(err.Error(), "deadlock") ||
		strings.Contains(err.Error(), "lock contention") ||
		strings.Contains(err.Error(), "resource unavailable") ||
		strings.Contains(err.Error(), "table is busy") ||
		strings.Contains(err.Error(), "transaction was deadlocked") {
		return true
	}

	return false
}

func IsRetryableError(err error) bool {
	if err == nil {
		return false
	}
	if err == odbc.ErrNotConnected {
		return true
	}
	return IsTransientError(err)
}

func calculateBackoff(attempt int, cfg *Config) time.Duration {
	interval := float64(cfg.InitialInterval) * math.Pow(cfg.Multiplier, float64(attempt-1))
	maxInterval := float64(cfg.MaxInterval)
	if interval > maxInterval {
		interval = maxInterval
	}
	jitter := (rand.Float64() * 0.3 * interval)
	return time.Duration(interval + jitter)
}

func Do(ctx context.Context, cfg *Config, op func() error) error {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	if cfg.MaxAttempts <= 0 {
		cfg.MaxAttempts = DefaultMaxAttempts
	}

	var lastErr error
	for attempt := 1; attempt <= cfg.MaxAttempts; attempt++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		lastErr = op()
		if lastErr == nil {
			return nil
		}

		if !IsRetryableError(lastErr) {
			return lastErr
		}

		if attempt < cfg.MaxAttempts {
			backoff := calculateBackoff(attempt, cfg)
			if cfg.OnRetry != nil {
				cfg.OnRetry(attempt, lastErr)
			}
			sleepCtx, cancel := context.WithTimeout(ctx, backoff)
			<-sleepCtx.Done()
			cancel()
			if sleepCtx.Err() != nil && sleepCtx.Err() != context.DeadlineExceeded {
				return sleepCtx.Err()
			}
		}
	}

	return lastErr
}

func DoWithResult(ctx context.Context, cfg *Config, op func() (interface{}, error)) (interface{}, error) {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	var lastErr error
	for attempt := 1; attempt <= cfg.MaxAttempts; attempt++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		result, err := op()
		if err == nil {
			return result, nil
		}

		lastErr = err
		if !IsRetryableError(err) {
			return nil, err
		}

		if attempt < cfg.MaxAttempts {
			backoff := calculateBackoff(attempt, cfg)
			if cfg.OnRetry != nil {
				cfg.OnRetry(attempt, err)
			}
			sleepCtx, cancel := context.WithTimeout(ctx, backoff)
			<-sleepCtx.Done()
			cancel()
			if sleepCtx.Err() != nil && sleepCtx.Err() != context.DeadlineExceeded {
				return nil, sleepCtx.Err()
			}
		}
	}

	return nil, lastErr
}

type RetryableFunc func() error

func UntilSuccess(ctx context.Context, maxAttempts int, fn RetryableFunc) error {
	return Do(ctx, &Config{MaxAttempts: maxAttempts}, fn)
}

func ConnectWithRetry(ctx context.Context, connString string) (*odbc.Connection, error) {
	var conn *odbc.Connection
	var err error

	err = Do(ctx, nil, func() error {
		conn, err = odbc.Connect(connString)
		return err
	})

	if err != nil {
		return nil, fmt.Errorf("connect with retry failed: %w", err)
	}

	return conn, nil
}

func ExecuteWithRetry(ctx context.Context, connString, sql string) (interface{}, error) {
	var result interface{}
	var err error

	err = Do(ctx, nil, func() error {
		conn, connErr := odbc.Connect(connString)
		if connErr != nil {
			return connErr
		}
		defer conn.Close()

		err = conn.ExecuteMultipleStatements(ctx, sql)
		return err
	})

	if err != nil {
		return nil, fmt.Errorf("execute with retry failed: %w", err)
	}

	return result, nil
}
