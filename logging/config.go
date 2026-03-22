package logging

import (
	"fmt"
	"os"
	"strings"
)

type LogFormat string

const (
	LogFormatJSON   LogFormat = "json"
	LogFormatPretty LogFormat = "pretty"
)

type LogLevel string

const (
	LogLevelDebug LogLevel = "debug"
	LogLevelInfo  LogLevel = "info"
	LogLevelWarn  LogLevel = "warn"
	LogLevelError LogLevel = "error"
	LogLevelTrace LogLevel = "trace"
)

type Config struct {
	LogFormat     LogFormat `json:"log_format"`
	LogLevel      LogLevel  `json:"log_level"`
	LogComponents []string  `json:"log_components"`
}

func (c *Config) Validate() error {
	switch c.LogFormat {
	case LogFormatJSON, LogFormatPretty, "":
	default:
		return fmt.Errorf("invalid log_format: %s (must be 'json' or 'pretty')", c.LogFormat)
	}

	switch c.LogLevel {
	case LogLevelDebug, LogLevelInfo, LogLevelWarn, LogLevelError, LogLevelTrace, "":
	default:
		return fmt.Errorf("invalid log_level: %s (must be 'debug', 'info', 'warn', 'error', or 'trace')", c.LogLevel)
	}

	return nil
}

func (c *Config) SetDefaults() {
	if c.LogFormat == "" {
		c.LogFormat = LogFormatJSON
	}
	if c.LogLevel == "" {
		c.LogLevel = LogLevelInfo
	}
	if c.LogComponents == nil {
		c.LogComponents = []string{}
	}
}

func (c *Config) IsComponentEnabled(component string) bool {
	if len(c.LogComponents) == 0 {
		return true
	}
	component = strings.ToLower(component)
	for _, c := range c.LogComponents {
		if strings.ToLower(c) == component {
			return true
		}
	}
	return false
}

func DefaultConfig() *Config {
	return &Config{
		LogFormat:     LogFormatJSON,
		LogLevel:      LogLevelInfo,
		LogComponents: []string{},
	}
}

func ConfigFromEnv() *Config {
	cfg := DefaultConfig()

	if format := os.Getenv("LOG_FORMAT"); format != "" {
		cfg.LogFormat = LogFormat(format)
	}
	if level := os.Getenv("LOG_LEVEL"); level != "" {
		cfg.LogLevel = LogLevel(level)
	}
	if components := os.Getenv("LOG_COMPONENTS"); components != "" {
		cfg.LogComponents = strings.Split(components, ",")
		for i, c := range cfg.LogComponents {
			cfg.LogComponents[i] = strings.TrimSpace(c)
		}
	}

	cfg.SetDefaults()
	return cfg
}

func ParseLogLevel(level string) (LogLevel, error) {
	switch strings.ToLower(level) {
	case "debug":
		return LogLevelDebug, nil
	case "info":
		return LogLevelInfo, nil
	case "warn", "warning":
		return LogLevelWarn, nil
	case "error":
		return LogLevelError, nil
	case "trace":
		return LogLevelTrace, nil
	default:
		return LogLevelInfo, fmt.Errorf("unknown log level: %s", level)
	}
}

func (l LogLevel) ToSlogLevel() int {
	switch l {
	case LogLevelTrace:
		return -8
	case LogLevelDebug:
		return -4
	case LogLevelInfo:
		return 0
	case LogLevelWarn:
		return 4
	case LogLevelError:
		return 8
	default:
		return 0
	}
}
