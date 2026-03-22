package logging

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"
)

var (
	defaultLogger *slog.Logger
	config        *Config
	configMu      sync.RWMutex
)

type contextKey string

const loggerKey contextKey = "logger"

const (
	ComponentBackend    = "backend"
	ComponentStorage    = "storage"
	ComponentODBC       = "odbc"
	ComponentCredential = "credential"
	ComponentRole       = "role"
	ComponentHealth     = "health"
	ComponentPool       = "pool"
	ComponentConfig     = "config"
	ComponentRotation   = "rotation"
	ComponentAudit      = "audit"
	ComponentWebhook    = "webhook"
	ComponentRateLimit  = "ratelimit"
	ComponentCache      = "cache"
	ComponentRetry      = "retry"
	ComponentMetrics    = "metrics"
	ComponentTracing    = "tracing"
)

type componentFilter struct {
	components map[string]bool
	mu         sync.RWMutex
}

var filter *componentFilter

func init() {
	filter = &componentFilter{
		components: make(map[string]bool),
	}
}

func Init(cfg *Config) error {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	if err := cfg.Validate(); err != nil {
		return err
	}

	cfg.SetDefaults()

	configMu.Lock()
	config = cfg
	configMu.Unlock()

	filter.mu.Lock()
	filter.components = make(map[string]bool)
	for _, c := range cfg.LogComponents {
		filter.components[strings.ToLower(c)] = true
	}
	filter.mu.Unlock()

	var handler slog.Handler
	var output io.Writer = os.Stdout

	opts := &slog.HandlerOptions{
		Level:     slog.Level(cfg.LogLevel.ToSlogLevel()),
		AddSource: false,
	}

	switch cfg.LogFormat {
	case LogFormatJSON:
		handler = slog.NewJSONHandler(output, opts)
	case LogFormatPretty:
		opts.AddSource = true
		handler = slog.NewTextHandler(output, opts)
	default:
		handler = slog.NewJSONHandler(output, opts)
	}

	defaultLogger = slog.New(handler)
	slog.SetDefault(defaultLogger)

	return nil
}

func InitFromEnv() error {
	return Init(ConfigFromEnv())
}

func UpdateConfig(cfg *Config) error {
	return Init(cfg)
}

func GetConfig() *Config {
	configMu.RLock()
	defer configMu.RUnlock()
	if config == nil {
		return DefaultConfig()
	}
	return config
}

func Default() *slog.Logger {
	configMu.RLock()
	initialized := defaultLogger != nil
	configMu.RUnlock()

	if !initialized {
		Init(nil)
	}
	return defaultLogger
}

func FromContext(ctx context.Context) *slog.Logger {
	if ctx == nil {
		return Default()
	}
	if logger, ok := ctx.Value(loggerKey).(*slog.Logger); ok && logger != nil {
		return logger
	}
	return Default()
}

func WithContext(ctx context.Context, logger *slog.Logger) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if logger == nil {
		return ctx
	}
	return context.WithValue(ctx, loggerKey, logger)
}

func WithComponent(ctx context.Context, component string, args ...any) *slog.Logger {
	logger := FromContext(ctx)
	if !IsComponentEnabled(component) {
		return &slog.Logger{}
	}
	return logger.With(append(args, slog.String("component", component))...)
}

func IsComponentEnabled(component string) bool {
	filter.mu.RLock()
	defer filter.mu.RUnlock()

	if len(filter.components) == 0 {
		return true
	}
	return filter.components[strings.ToLower(component)]
}

func Debug(msg string, args ...any) {
	Default().Debug(msg, args...)
}

func Info(msg string, args ...any) {
	Default().Info(msg, args...)
}

func Warn(msg string, args ...any) {
	Default().Warn(msg, args...)
}

func Error(msg string, args ...any) {
	Default().Error(msg, args...)
}

func Log(level LogLevel, msg string, args ...any) {
	switch level {
	case LogLevelTrace:
		Default().Log(context.Background(), slog.Level(-8), msg, args...)
	case LogLevelDebug:
		Default().Debug(msg, args...)
	case LogLevelInfo:
		Default().Info(msg, args...)
	case LogLevelWarn:
		Default().Warn(msg, args...)
	case LogLevelError:
		Default().Error(msg, args...)
	}
}

type HealthCheckResult struct {
	Component      string        `json:"component"`
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
		slog.String("component", h.Component),
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

	args := []any{
		slog.String("component", ComponentHealth),
		slog.String("connection_name", connName),
		slog.String("state", state),
		slog.Duration("latency", latency),
		slog.Time("timestamp", time.Now()),
	}

	if err != nil {
		args = append(args, slog.String("error", err.Error()))
		logger.Error("health_check", args...)
	} else {
		logger.Info("health_check", args...)
	}
}

func LogConnectionEvent(logger *slog.Logger, event string, connName string, details map[string]interface{}) {
	if logger == nil {
		logger = Default()
	}

	args := []any{
		slog.String("component", ComponentPool),
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
		slog.String("component", ComponentPool),
		slog.String("connection_name", connName),
		slog.Int("open_connections", openConns),
		slog.Int("idle_connections", idleConns),
		slog.Int("in_use_connections", inUse),
		slog.Time("timestamp", time.Now()),
	)
}

func LogOperation(logger *slog.Logger, component, operation string, args ...any) {
	if logger == nil {
		logger = Default()
	}

	operationArgs := append([]any{
		slog.String("component", component),
		slog.String("operation", operation),
		slog.Time("timestamp", time.Now()),
	}, args...)

	logger.Info("operation", operationArgs...)
}

func LogError(logger *slog.Logger, component, operation string, err error, args ...any) {
	if logger == nil {
		logger = Default()
	}

	errArgs := append([]any{
		slog.String("component", component),
		slog.String("operation", operation),
		slog.String("error", err.Error()),
		slog.Time("timestamp", time.Now()),
	}, args...)

	logger.Error("operation_error", errArgs...)
}

func LogCredentialOperation(logger *slog.Logger, operation string, role string, username string, args ...any) {
	if logger == nil {
		logger = Default()
	}

	credArgs := append([]any{
		slog.String("component", ComponentCredential),
		slog.String("operation", operation),
		slog.String("role", role),
		slog.String("username", username),
		slog.Time("timestamp", time.Now()),
	}, args...)

	logger.Info("credential_operation", credArgs...)
}

func LogRoleOperation(logger *slog.Logger, operation string, role string, args ...any) {
	if logger == nil {
		logger = Default()
	}

	roleArgs := append([]any{
		slog.String("component", ComponentRole),
		slog.String("operation", operation),
		slog.String("role", role),
		slog.Time("timestamp", time.Now()),
	}, args...)

	logger.Info("role_operation", roleArgs...)
}

func LogConfigOperation(logger *slog.Logger, operation string, args ...any) {
	if logger == nil {
		logger = Default()
	}

	configArgs := append([]any{
		slog.String("component", ComponentConfig),
		slog.String("operation", operation),
		slog.Time("timestamp", time.Now()),
	}, args...)

	logger.Info("config_operation", configArgs...)
}

func LogRotationEvent(logger *slog.Logger, operation string, username string, args ...any) {
	if logger == nil {
		logger = Default()
	}

	rotArgs := append([]any{
		slog.String("component", ComponentRotation),
		slog.String("operation", operation),
		slog.String("username", username),
		slog.Time("timestamp", time.Now()),
	}, args...)

	logger.Info("rotation_event", rotArgs...)
}

func LogDebug(logger *slog.Logger, component string, msg string, args ...any) {
	if logger == nil {
		logger = Default()
	}

	debugArgs := append([]any{
		slog.String("component", component),
		slog.String("message", msg),
	}, args...)

	logger.Debug("debug", debugArgs...)
}

func GetCallerLocation() (string, int) {
	_, file, line, ok := runtime.Caller(2)
	if !ok {
		return "", 0
	}
	parts := strings.Split(file, "/")
	if len(parts) > 2 {
		file = strings.Join(parts[len(parts)-2:], "/")
	}
	return file, line
}

type structuredLogEntry struct {
	Time           string                 `json:"time"`
	Level          string                 `json:"level"`
	Message        string                 `json:"msg"`
	Component      string                 `json:"component,omitempty"`
	Operation      string                 `json:"operation,omitempty"`
	ConnectionName string                 `json:"connection_name,omitempty"`
	Error          string                 `json:"error,omitempty"`
	Fields         map[string]interface{} `json:"fields,omitempty"`
}

func ParseJSONLogEntry(data []byte) (*structuredLogEntry, error) {
	var entry structuredLogEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil, err
	}
	return &entry, nil
}

func ValidateJSONOutput(data []byte) bool {
	var m map[string]interface{}
	return json.Valid(data) && json.Unmarshal(data, &m) == nil
}
