package logging

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"
)

const (
	LevelTrace slog.Level = -8
)

var (
	defaultLogger   *slog.Logger
	globalConfig    *Config
	componentLevels = make(map[string]slog.Level)
)

type contextKey string

const loggerKey contextKey = "logger"

type componentHandler struct {
	slog.Handler
	component string
}

func (h *componentHandler) Handle(ctx context.Context, r slog.Record) error {
	if globalConfig != nil && !globalConfig.IsComponentEnabled(h.component) {
		return nil
	}
	r.AddAttrs(slog.String("component", h.component))
	return h.Handler.Handle(ctx, r)
}

func Init(cfg *Config) {
	if cfg == nil {
		defaultCfg := DefaultConfig()
		cfg = &defaultCfg
	}
	globalConfig = cfg

	var logLevel slog.Level
	switch strings.ToLower(string(cfg.Level)) {
	case string(LogLevelTrace):
		logLevel = LevelTrace
	case string(LogLevelDebug):
		logLevel = slog.LevelDebug
	case string(LogLevelWarn):
		logLevel = slog.LevelWarn
	case string(LogLevelError):
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		Level: logLevel,
	}

	var handler slog.Handler
	if cfg.Format == LogFormatPretty {
		handler = slog.NewTextHandler(os.Stdout, opts)
	} else {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	}

	defaultLogger = slog.New(handler)
	slog.SetDefault(defaultLogger)
}

func InitWithFormat(level string, format LogFormat) {
	Init(&Config{
		Format: format,
		Level:  LogLevel(level),
	})
}

func SetComponentLevel(component string, level LogLevel) {
	var lvl slog.Level
	switch strings.ToLower(string(level)) {
	case string(LogLevelTrace):
		lvl = LevelTrace
	case string(LogLevelDebug):
		lvl = slog.LevelDebug
	case string(LogLevelWarn):
		lvl = slog.LevelWarn
	case string(LogLevelError):
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}
	componentLevels[strings.ToLower(component)] = lvl
}

func GetConfig() *Config {
	if globalConfig == nil {
		defaultCfg := DefaultConfig()
		return &defaultCfg
	}
	return globalConfig
}

func Default() *slog.Logger {
	if defaultLogger == nil {
		Init(nil)
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

func WithComponent(ctx context.Context, component string) context.Context {
	logger := FromContext(ctx)
	cfg := GetConfig()
	if !cfg.IsComponentEnabled(component) {
		return ctx
	}

	componentLogger := logger.With(
		slog.String("component", component),
	)
	return context.WithValue(ctx, loggerKey, componentLogger)
}

func WithFields(ctx context.Context, fields ...any) context.Context {
	logger := FromContext(ctx)
	componentLogger := logger.With(fields...)
	return context.WithValue(ctx, loggerKey, componentLogger)
}

type HealthCheckResult struct {
	ConnectionName string        `json:"connection_name"`
	State          string        `json:"state"`
	Latency        time.Duration `json:"latency"`
	Error          string        `json:"error,omitempty"`
	Timestamp      time.Time     `json:"timestamp"`
}

func (h *HealthCheckResult) Log(logger *slog.Logger) {
	if logger == nil {
		logger = Default()
	}
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

	if err != nil {
		logger.Info("health_check",
			slog.String("connection_name", connName),
			slog.String("state", state),
			slog.Duration("latency", latency),
			slog.Time("timestamp", time.Now()),
			slog.String("error", err.Error()),
		)
	} else {
		logger.Info("health_check",
			slog.String("connection_name", connName),
			slog.String("state", state),
			slog.Duration("latency", latency),
			slog.Time("timestamp", time.Now()),
		)
	}
}

func LogConnectionEvent(logger *slog.Logger, event string, connName string, details map[string]any) {
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

func LogOperation(logger *slog.Logger, component, operation string, fields ...any) {
	if logger == nil {
		logger = Default()
	}

	args := []any{
		slog.String("component", component),
		slog.String("operation", operation),
		slog.Time("timestamp", time.Now()),
	}
	args = append(args, fields...)

	logger.Info("operation", args...)
}

func LogCredentialOperation(logger *slog.Logger, component, operation, role, username string, fields ...any) {
	if logger == nil {
		logger = Default()
	}

	args := []any{
		slog.String("component", component),
		slog.String("operation", operation),
		slog.String("role", role),
		slog.String("username", username),
		slog.Time("timestamp", time.Now()),
	}
	args = append(args, fields...)

	logger.Info("credential_operation", args...)
}

func LogDatabaseOperation(logger *slog.Logger, component, operation, database string, fields ...any) {
	if logger == nil {
		logger = Default()
	}

	args := []any{
		slog.String("component", component),
		slog.String("operation", operation),
		slog.String("database", database),
		slog.Time("timestamp", time.Now()),
	}
	args = append(args, fields...)

	logger.Info("database_operation", args...)
}

func LogError(logger *slog.Logger, component, operation string, err error, fields ...any) {
	if logger == nil {
		logger = Default()
	}

	args := []any{
		slog.String("component", component),
		slog.String("operation", operation),
		slog.String("error", err.Error()),
		slog.Time("timestamp", time.Now()),
	}
	args = append(args, fields...)

	logger.Error("error", args...)
}

func LogDebug(logger *slog.Logger, component, operation string, fields ...any) {
	if logger == nil {
		logger = Default()
	}

	args := []any{
		slog.String("component", component),
		slog.String("operation", operation),
		slog.Time("timestamp", time.Now()),
	}
	args = append(args, fields...)

	logger.Debug("debug", args...)
}

func LogTrace(logger *slog.Logger, component, operation string, fields ...any) {
	if logger == nil {
		logger = Default()
	}

	args := []any{
		slog.String("component", component),
		slog.String("operation", operation),
		slog.Time("timestamp", time.Now()),
	}
	args = append(args, fields...)

	logger.Log(context.Background(), LevelTrace, "trace", args...)
}

func NewJSONLogger(w io.Writer, level string) *slog.Logger {
	var logLevel slog.Level
	switch strings.ToLower(level) {
	case "debug":
		logLevel = slog.LevelDebug
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	case "trace":
		logLevel = LevelTrace
	default:
		logLevel = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{Level: logLevel}
	handler := slog.NewJSONHandler(w, opts)
	return slog.New(handler)
}

func NewPrettyLogger(w io.Writer, level string) *slog.Logger {
	var logLevel slog.Level
	switch strings.ToLower(level) {
	case "debug":
		logLevel = slog.LevelDebug
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	case "trace":
		logLevel = LevelTrace
	default:
		logLevel = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{Level: logLevel}
	handler := slog.NewTextHandler(w, opts)
	return slog.New(handler)
}
