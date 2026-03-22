package logging

import (
	"strings"
)

type LogFormat string

const (
	LogFormatJSON   LogFormat = "json"
	LogFormatPretty LogFormat = "pretty"
)

type LogLevel string

const (
	LogLevelTrace LogLevel = "trace"
	LogLevelDebug LogLevel = "debug"
	LogLevelInfo  LogLevel = "info"
	LogLevelWarn  LogLevel = "warn"
	LogLevelError LogLevel = "error"
)

type Config struct {
	Format     LogFormat `json:"format" env:"LOG_FORMAT"`
	Level      LogLevel  `json:"level" env:"LOG_LEVEL"`
	Components []string  `json:"components,omitempty" env:"LOG_COMPONENTS"`
}

func DefaultConfig() Config {
	return Config{
		Format: LogFormatJSON,
		Level:  LogLevelInfo,
	}
}

func (c *Config) Validate() error {
	switch strings.ToLower(string(c.Format)) {
	case string(LogFormatJSON), string(LogFormatPretty), "":
		c.Format = LogFormatJSON
	default:
		c.Format = LogFormatJSON
	}

	switch strings.ToLower(string(c.Level)) {
	case string(LogLevelTrace), string(LogLevelDebug), string(LogLevelInfo),
		string(LogLevelWarn), string(LogLevelError), "":
		if c.Level == "" {
			c.Level = LogLevelInfo
		}
	default:
		c.Level = LogLevelInfo
	}

	return nil
}

func (c *Config) IsComponentEnabled(component string) bool {
	if len(c.Components) == 0 {
		return true
	}

	component = strings.ToLower(component)
	for _, comp := range c.Components {
		if strings.ToLower(comp) == component {
			return true
		}
	}
	return false
}

func ParseConfig(format, level string, components []string) (*Config, error) {
	cfg := &Config{
		Format:     LogFormat(format),
		Level:      LogLevel(level),
		Components: components,
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}
