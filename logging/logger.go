package logging

import (
	"context"
	"log/slog"
	"os"
	"time"
)

var defaultLogger *slog.Logger

type contextKey string

const loggerKey contextKey = "logger"

func Init(level string) {
	var logLevel slog.Level
	switch level {
	case "debug":
		logLevel = slog.LevelDebug
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		Level: logLevel,
	}

	handler := slog.NewJSONHandler(os.Stdout, opts)
	defaultLogger = slog.New(handler)
	slog.SetDefault(defaultLogger)
}

func Default() *slog.Logger {
	if defaultLogger == nil {
		Init("info")
	}
	return defaultLogger
}

func FromContext(ctx context.Context) *slog.Logger {
	if logger, ok := ctx.Value(loggerKey).(*slog.Logger); ok && logger != nil {
		return logger
	}
	return Default()
}

func WithContext(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, loggerKey, logger)
}

type HealthCheckResult struct {
	ConnectionName string        `json:"connection_name"`
	State          string        `json:"state"`
	Latency        time.Duration `json:"latency"`
	Error          string        `json:"error,omitempty"`
	Timestamp      time.Time     `json:"timestamp"`
}

func (h *HealthCheckResult) Log(logger *slog.Logger) {
	logger.Info("health_check_result",
		slog.String("connection_name", h.ConnectionName),
		slog.String("state", h.State),
		slog.Duration("latency", h.Latency),
		slog.String("error", h.Error),
		slog.Time("timestamp", h.Timestamp),
	)
}

func LogHealthCheck(logger *slog.Logger, connName string, state string, latency time.Duration, err error) {
	if logger == nil {
		logger = Default()
	}

	logger.Info("health_check",
		slog.String("connection_name", connName),
		slog.String("state", state),
		slog.Duration("latency", latency),
		slog.Time("timestamp", time.Now()),
		slog.String("error", ""),
	)

	if err != nil {
		logger.Info("health_check",
			slog.String("connection_name", connName),
			slog.String("state", state),
			slog.Duration("latency", latency),
			slog.Time("timestamp", time.Now()),
			slog.String("error", err.Error()),
		)
	}
}

func LogConnectionEvent(logger *slog.Logger, event string, connName string, details map[string]interface{}) {
	if logger == nil {
		logger = Default()
	}

	args := []any{
		slog.String("event", event),
		slog.String("connection_name", connName),
		slog.Time("timestamp", time.Now()),
	}

	for k, v := range details {
		args = append(args, slog.Any(k, v))
	}

	logger.Info("connection_event", args...)
}

func LogPoolStats(logger *slog.Logger, connName string, openConns int, idleConns int, inUse int) {
	if logger == nil {
		logger = Default()
	}

	logger.Info("pool_stats",
		slog.String("connection_name", connName),
		slog.Int("open_connections", openConns),
		slog.Int("idle_connections", idleConns),
		slog.Int("in_use_connections", inUse),
		slog.Time("timestamp", time.Now()),
	)
}
