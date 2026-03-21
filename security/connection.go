package security

import (
	"errors"
	"fmt"
	"strings"
	"unicode"
)

var (
	ErrEmptyConnectionString = errors.New("connection string cannot be empty")
	ErrInvalidFormat         = errors.New("invalid connection string format")
	ErrMissingDSN            = errors.New("connection string must contain a DSN or SERVER parameter")
	ErrInvalidKeyValue       = errors.New("invalid key-value pair in connection string")
)

var sensitiveKeywords = []string{
	"password",
	"pwd",
	"pass",
	"secret",
	"token",
	"auth",
	"credential",
	"key",
	"private",
}

var requiredKeywords = []string{
	"dsn",
	"server",
	"servers",
}

func ValidateConnectionString(connString string) error {
	if strings.TrimSpace(connString) == "" {
		return ErrEmptyConnectionString
	}

	connString = strings.TrimSpace(connString)

	hasRequired := false
	for _, req := range requiredKeywords {
		lowerConn := strings.ToLower(connString)
		if strings.Contains(lowerConn, req+"=") {
			hasRequired = true
			break
		}
	}

	if !hasRequired {
		return ErrMissingDSN
	}

	pairs, err := parseConnectionString(connString)
	if err != nil {
		return err
	}

	if len(pairs) == 0 {
		return ErrInvalidFormat
	}

	for key, value := range pairs {
		if err := validateKeyValuePair(key, value); err != nil {
			return err
		}
	}

	return nil
}

func parseConnectionString(connString string) (map[string]string, error) {
	pairs := make(map[string]string)
	var currentKey strings.Builder
	var currentValue strings.Builder
	var inValue bool
	var inQuote bool
	escaped := false

	for i, ch := range connString {
		if escaped {
			if inValue {
				currentValue.WriteRune(ch)
			}
			escaped = false
			continue
		}

		if ch == '\\' && i+1 < len(connString) {
			escaped = true
			continue
		}

		if ch == '"' {
			if !inQuote && !inValue {
				return nil, ErrInvalidFormat
			}
			inQuote = !inQuote
			continue
		}

		if ch == '=' && !inQuote && !inValue {
			inValue = true
			continue
		}

		if (ch == ';' || ch == '\n' || ch == '\r') && !inQuote && inValue {
			key := strings.TrimSpace(currentKey.String())
			value := strings.TrimSpace(currentValue.String())

			if key != "" {
				pairs[strings.ToLower(key)] = value
			}

			currentKey.Reset()
			currentValue.Reset()
			inValue = false
			continue
		}

		if unicode.IsSpace(ch) && !inQuote && !inValue {
			continue
		}

		if inValue {
			currentValue.WriteRune(ch)
		} else {
			currentKey.WriteRune(ch)
		}
	}

	if currentKey.Len() > 0 {
		key := strings.TrimSpace(currentKey.String())
		value := strings.TrimSpace(currentValue.String())
		if key != "" {
			pairs[strings.ToLower(key)] = value
		}
	}

	return pairs, nil
}

func validateKeyValuePair(key, value string) error {
	if key == "" {
		return ErrInvalidKeyValue
	}

	if strings.Contains(key, " ") {
		return fmt.Errorf("%w: key '%s' contains spaces", ErrInvalidKeyValue, key)
	}

	for _, r := range key {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' && r != '-' {
			return fmt.Errorf("%w: key '%s' contains invalid character '%c'", ErrInvalidKeyValue, key, r)
		}
	}

	return nil
}

func MaskConnectionString(connString string) string {
	if connString == "" {
		return ""
	}

	pairs, err := parseConnectionString(connString)
	if err != nil {
		return "***"
	}

	var result []string
	for key, value := range pairs {
		if isSensitiveKey(key) {
			result = append(result, fmt.Sprintf("%s=***", key))
		} else {
			if value != "" {
				result = append(result, fmt.Sprintf("%s=%s", key, value))
			} else {
				result = append(result, key)
			}
		}
	}

	return strings.Join(result, ";")
}

func isSensitiveKey(key string) bool {
	lowerKey := strings.ToLower(key)
	for _, sensitive := range sensitiveKeywords {
		if strings.Contains(lowerKey, sensitive) {
			return true
		}
	}
	return false
}

func GetConnectionStringInfo(connString string) (hasCredentials bool, hasServer bool, err error) {
	if connString == "" {
		return false, false, nil
	}

	pairs, err := parseConnectionString(connString)
	if err != nil {
		return false, false, err
	}

	hasCredentials = false
	hasServer = false

	for key := range pairs {
		if isSensitiveKey(key) {
			hasCredentials = true
		}
		if key == "server" || key == "servers" || key == "dsn" {
			hasServer = true
		}
	}

	return hasCredentials, hasServer, nil
}
