package logging

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	LevelTrace = slog.Level(-8)
	LevelDebug = slog.LevelDebug
	LevelInfo  = slog.LevelInfo
	LevelWarn  = slog.LevelWarn
	LevelError = slog.LevelError
)

const (
	EnvLogLevel  = "TERADATA_PLUGIN_LOG_LEVEL"
	EnvLogFormat = "TERADATA_PLUGIN_LOG_FORMAT"
	EnvService   = "TERADATA_PLUGIN_SERVICE"
)

const (
	FormatJSON    = "json"
	FormatConsole = "console"
)

var defaultLogger *slog.Logger
var logLevel = LevelInfo
var logFormat = FormatJSON
var serviceName = "teradata-secret-plugin"

type contextKey string

const loggerKey contextKey = "logger"

type LogLevel string

const (
	LogLevelTrace LogLevel = "trace"
	LogLevelDebug LogLevel = "debug"
	LogLevelInfo  LogLevel = "info"
	LogLevelWarn  LogLevel = "warn"
	LogLevelError LogLevel = "error"
)

type LoggerConfig struct {
	Level      string
	Format     string
	Service    string
	TimeFormat string
}

func Init(opts ...Option) {
	config := &LoggerConfig{
		Level:      getEnvOrDefault(EnvLogLevel, "info"),
		Format:     getEnvOrDefault(EnvLogFormat, FormatJSON),
		Service:    getEnvOrDefault(EnvService, serviceName),
		TimeFormat: time.RFC3339,
	}

	for _, opt := range opts {
		opt(config)
	}

	logLevel = parseLogLevel(config.Level)
	logFormat = config.Format
	serviceName = config.Service

	levelVal := &slog.LevelVar{}
	levelVal.Set(logLevel)

	optsStruct := &slog.HandlerOptions{
		Level:     levelVal,
		AddSource: logLevel == LevelTrace,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey && config.TimeFormat != "" {
				if t, ok := a.Value.Any().(time.Time); ok {
					a.Value = slog.StringValue(t.Format(config.TimeFormat))
				}
			}
			if a.Key == slog.LevelKey {
				switch a.Value.Any().(slog.Level) {
				case LevelTrace:
					a.Value = slog.StringValue("TRACE")
				case LevelDebug:
					a.Value = slog.StringValue("DEBUG")
				case LevelInfo:
					a.Value = slog.StringValue("INFO")
				case LevelWarn:
					a.Value = slog.StringValue("WARN")
				case LevelError:
					a.Value = slog.StringValue("ERROR")
				}
			}
			return a
		},
	}

	var handler slog.Handler
	if config.Format == FormatConsole {
		handler = slog.NewTextHandler(os.Stdout, optsStruct)
	} else {
		handler = slog.NewJSONHandler(os.Stdout, optsStruct)
	}

	defaultLogger = slog.New(handler)
	defaultLogger = defaultLogger.With(
		slog.String("service", serviceName),
	)
	slog.SetDefault(defaultLogger)
}

type Option func(*LoggerConfig)

func WithLevel(level string) Option {
	return func(c *LoggerConfig) {
		c.Level = level
	}
}

func WithFormat(format string) Option {
	return func(c *LoggerConfig) {
		c.Format = format
	}
}

func WithService(service string) Option {
	return func(c *LoggerConfig) {
		c.Service = service
	}
}

func WithTimeFormat(format string) Option {
	return func(c *LoggerConfig) {
		c.TimeFormat = format
	}
}

func parseLogLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "trace":
		return LevelTrace
	case "debug":
		return LevelDebug
	case "warn", "warning":
		return LevelWarn
	case "error":
		return LevelError
	case "info":
		return LevelInfo
	default:
		return LevelInfo
	}
}

func getEnvOrDefault(env, defaultVal string) string {
	if val := os.Getenv(env); val != "" {
		return val
	}
	return defaultVal
}

func GetLevel() slog.Level {
	return logLevel
}

func GetFormat() string {
	return logFormat
}

func GetService() string {
	return serviceName
}

func SetLevel(level string) {
	logLevel = parseLogLevel(level)
}

func Default() *slog.Logger {
	if defaultLogger == nil {
		Init()
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
	if logger == nil {
		logger = Default()
	}

	attrs := []any{
		slog.String("connection_name", h.ConnectionName),
		slog.String("state", h.State),
		slog.Duration("latency", h.Latency),
		slog.String("error", h.Error),
		slog.Time("timestamp", h.Timestamp),
		slog.String("component", "health_check"),
	}

	switch h.State {
	case "healthy":
		logger.Info("health_check_result", attrs...)
	case "unhealthy", "closed":
		logger.Error("health_check_result", attrs...)
	default:
		logger.Warn("health_check_result", attrs...)
	}
}

func LogHealthCheck(logger *slog.Logger, connName string, state string, latency time.Duration, err error) {
	if logger == nil {
		logger = Default()
	}

	attrs := []any{
		slog.String("connection_name", connName),
		slog.String("state", state),
		slog.Duration("latency", latency),
		slog.Time("timestamp", time.Now()),
		slog.String("component", "health_check"),
	}

	if err != nil {
		attrs = append(attrs, slog.String("error", err.Error()))
		logger.Error("health_check", attrs...)
		return
	}

	switch state {
	case "healthy":
		logger.Info("health_check", attrs...)
	case "unhealthy", "closed":
		logger.Warn("health_check", attrs...)
	default:
		logger.Debug("health_check", attrs...)
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
		slog.String("component", "connection"),
	}

	for k, v := range details {
		args = append(args, slog.Any(k, v))
	}

	switch event {
	case "prewarm_pool_error", "prewarm_config_load_error", "prewarm_config_decode_error",
		"connection_failed", "connection_error", "pool_error":
		logger.Error("connection_event", args...)
	case "idle_connection_closed", "connection_removed", "connection_closed":
		logger.Warn("connection_event", args...)
	case "pool_prewarmed", "connection_added", "connection_updated", "pool_warmup_started", "pool_warmup_completed":
		logger.Info("connection_event", args...)
	default:
		logger.Debug("connection_event", args...)
	}
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
		slog.String("component", "pool"),
	)
}

func LogCredentialEvent(logger *slog.Logger, event string, username string, roleName string, details map[string]any) {
	if logger == nil {
		logger = Default()
	}

	args := []any{
		slog.String("event", event),
		slog.String("username", username),
		slog.String("role_name", roleName),
		slog.Time("timestamp", time.Now()),
		slog.String("component", "credential"),
	}

	for k, v := range details {
		args = append(args, slog.Any(k, v))
	}

	switch event {
	case "credential_created", "credential_renewed":
		logger.Info("credential_event", args...)
	case "credential_revoked", "credential_expired":
		logger.Info("credential_event", args...)
	case "credential_error", "credential_creation_failed", "credential_revocation_failed":
		logger.Error("credential_event", args...)
	default:
		logger.Debug("credential_event", args...)
	}
}

func LogRoleEvent(logger *slog.Logger, event string, roleName string, details map[string]any) {
	if logger == nil {
		logger = Default()
	}

	args := []any{
		slog.String("event", event),
		slog.String("role_name", roleName),
		slog.Time("timestamp", time.Now()),
		slog.String("component", "role"),
	}

	for k, v := range details {
		args = append(args, slog.Any(k, v))
	}

	switch event {
	case "role_created", "role_updated":
		logger.Info("role_event", args...)
	case "role_deleted":
		logger.Info("role_event", args...)
	case "role_error", "role_creation_failed":
		logger.Error("role_event", args...)
	default:
		logger.Debug("role_event", args...)
	}
}

func LogConfigEvent(logger *slog.Logger, event string, region string, details map[string]any) {
	if logger == nil {
		logger = Default()
	}

	args := []any{
		slog.String("event", event),
		slog.String("region", region),
		slog.Time("timestamp", time.Now()),
		slog.String("component", "config"),
	}

	for k, v := range details {
		args = append(args, slog.Any(k, v))
	}

	switch event {
	case "config_created", "config_updated":
		logger.Info("config_event", args...)
	case "config_deleted":
		logger.Info("config_event", args...)
	case "config_error", "config_validation_failed":
		logger.Error("config_event", args...)
	default:
		logger.Debug("config_event", args...)
	}
}

func LogStartup(logger *slog.Logger, version string) {
	if logger == nil {
		logger = Default()
	}
	logger.Info("plugin_startup",
		slog.String("version", version),
		slog.String("service", serviceName),
		slog.String("log_level", logLevel.String()),
		slog.String("log_format", logFormat),
		slog.Time("timestamp", time.Now()),
		slog.String("component", "startup"),
	)
}

func LogShutdown(logger *slog.Logger) {
	if logger == nil {
		logger = Default()
	}
	logger.Info("plugin_shutdown",
		slog.String("service", serviceName),
		slog.Time("timestamp", time.Now()),
		slog.String("component", "shutdown"),
	)
}

func LogError(logger *slog.Logger, msg string, err error, details map[string]any) {
	if logger == nil {
		logger = Default()
	}

	args := []any{
		slog.String("error", err.Error()),
		slog.Time("timestamp", time.Now()),
	}

	for k, v := range details {
		args = append(args, slog.Any(k, v))
	}

	logger.Error(msg, args...)
}

func SetLevelFromEnv() {
	if val := os.Getenv(EnvLogLevel); val != "" {
		SetLevel(val)
	}
}

func SetFormatFromEnv() {
	if val := os.Getenv(EnvLogFormat); val != "" {
		if val == FormatConsole || val == FormatJSON {
			logFormat = val
		}
	}
}

func SetServiceFromEnv() {
	if val := os.Getenv(EnvService); val != "" {
		serviceName = val
	}
}

func UpdateLogLevel(levelStr string) error {
	parsedLevel, err := strconv.Atoi(levelStr)
	if err != nil {
		SetLevel(levelStr)
		return nil
	}

	switch parsedLevel {
	case int(LevelTrace):
		SetLevel("trace")
	case int(LevelDebug):
		SetLevel("debug")
	case int(LevelInfo):
		SetLevel("info")
	case int(LevelWarn):
		SetLevel("warn")
	case int(LevelError):
		SetLevel("error")
	default:
		return fmt.Errorf("invalid log level: %d", parsedLevel)
	}
	return nil
}
