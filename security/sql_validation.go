package security

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

var (
	ErrEmptyStatement      = errors.New("SQL statement cannot be empty")
	ErrInvalidSQLStatement = errors.New("invalid SQL statement")
	ErrDangerousPattern    = errors.New("potentially dangerous SQL pattern detected")
	ErrStatementTooLong    = errors.New("SQL statement exceeds maximum length")
	ErrMultipleStatements  = errors.New("multiple statements not allowed")
	ErrCommentDetected     = errors.New("SQL comments not allowed")
	ErrInvalidPassword     = errors.New("password contains invalid characters")
	ErrPasswordTooLong     = errors.New("password exceeds maximum length")
	ErrInvalidSessionVar   = errors.New("invalid session variable")
	ErrSessionVarTooLong   = errors.New("session variable value exceeds maximum length")
)

const (
	MaxStatementLength    = 10000
	MaxPasswordLength     = 256
	MaxSessionVarNameLen  = 128
	MaxSessionVarValueLen = 1024
	MaxSessionVars        = 50
)

var sqlInjectionPatterns = []string{
	"--",
	"/*",
	"*/",
	"xp_",
	"sp_",
	"sys.",
	"sysobjects",
	"syscolumns",
	"waitfor",
	"delay",
	"0x",
	"char(",
	"nchar(",
	"varchar(",
	"nvarchar(",
	"exec(",
	"execute(",
	"eval(",
}

var dangerousStatementKeywords = []string{
	"SELECT",
	"INSERT",
	"UPDATE",
	"DELETE",
	"DROP",
	"CREATE",
	"ALTER",
	"TRUNCATE",
	"EXEC",
	"EXECUTE",
	"CALL",
	"DECLARE",
	"MERGE",
	"UNION",
	"INTO",
	"FROM",
	"WHERE",
	"JOIN",
	"GROUP BY",
	"ORDER BY",
}

var commentPatterns = []*regexp.Regexp{
	regexp.MustCompile(`--`),
	regexp.MustCompile(`/\*`),
}

var dangerousPatterns = []string{
	";",
}

var grantPrivilegeKeywords = []string{
	"SELECT",
	"INSERT",
	"UPDATE",
	"DELETE",
	"EXECUTE",
}

func detectStatementType(stmt string) StatementType {
	upper := strings.ToUpper(strings.TrimSpace(stmt))
	if strings.HasPrefix(upper, "GRANT") {
		return StatementTypeGRANT
	}
	if strings.HasPrefix(upper, "REVOKE") {
		return StatementTypeREVOKE
	}
	return StatementTypeUnknown
}

func ValidatePassword(password string) error {
	if password == "" {
		return ErrInvalidPassword
	}

	if len(password) > MaxPasswordLength {
		return ErrPasswordTooLong
	}

	for _, r := range password {
		if r < 32 || r > 126 {
			return ErrInvalidPassword
		}
		if !unicode.IsPrint(r) {
			return ErrInvalidPassword
		}
	}

	invalidChars := []string{"'", "\"", "\\", "\x00"}
	for _, char := range invalidChars {
		if strings.Contains(password, char) {
			return ErrInvalidPassword
		}
	}

	if strings.Contains(password, " ") {
		return ErrInvalidPassword
	}

	return nil
}

func ValidateSQLStatement(statement string) error {
	if strings.TrimSpace(statement) == "" {
		return ErrEmptyStatement
	}

	if len(statement) > MaxStatementLength {
		return ErrStatementTooLong
	}

	upperStatement := strings.ToUpper(statement)

	for _, pattern := range commentPatterns {
		if pattern.MatchString(statement) {
			return ErrCommentDetected
		}
	}

	for _, pattern := range sqlInjectionPatterns {
		if strings.Contains(upperStatement, strings.ToUpper(pattern)) {
			return fmt.Errorf("%w: found pattern '%s'", ErrDangerousPattern, pattern)
		}
	}

	stmtType := detectStatementType(statement)

	for _, pattern := range dangerousPatterns {
		if strings.Contains(upperStatement, strings.ToUpper(pattern)) {
			return fmt.Errorf("%w: found pattern '%s'", ErrDangerousPattern, pattern)
		}
	}

	if stmtType == StatementTypeGRANT || stmtType == StatementTypeREVOKE {
		skipKeywords := map[string]bool{
			"SELECT": true, "INSERT": true, "UPDATE": true, "DELETE": true, "EXECUTE": true,
			"FROM": true, "ON": true, "TO": true, "BY": true, "WITH": true, "GRANT": true, "REVOKE": true,
		}
		for _, keyword := range dangerousStatementKeywords {
			if skipKeywords[keyword] {
				continue
			}
			if strings.Contains(upperStatement, keyword) {
				idx := strings.Index(upperStatement, keyword)
				before := ""
				if idx > 0 {
					before = string(upperStatement[idx-1])
				}
				after := ""
				if idx+len(keyword) < len(upperStatement) {
					after = string(upperStatement[idx+len(keyword)])
				}
				isWordBoundary := (before == "" || !unicode.IsLetter([]rune(before)[0])) &&
					(after == "" || (!unicode.IsLetter([]rune(after)[0]) && after != "_" && after != "(" && after != ","))
				if isWordBoundary {
					return fmt.Errorf("%w: SQL keyword '%s' not allowed in user statements", ErrDangerousPattern, keyword)
				}
			}
		}
	} else {
		for _, keyword := range dangerousStatementKeywords {
			if strings.Contains(upperStatement, keyword) {
				idx := strings.Index(upperStatement, keyword)
				before := ""
				if idx > 0 {
					before = string(upperStatement[idx-1])
				}
				after := ""
				if idx+len(keyword) < len(upperStatement) {
					after = string(upperStatement[idx+len(keyword)])
				}
				isWordBoundary := (before == "" || !unicode.IsLetter([]rune(before)[0])) &&
					(after == "" || (!unicode.IsLetter([]rune(after)[0]) && after != "_" && after != "("))
				if isWordBoundary {
					return fmt.Errorf("%w: SQL keyword '%s' not allowed in user statements", ErrDangerousPattern, keyword)
				}
			}
		}
	}

	return nil
}

func ValidateUsername(username string) error {
	if username == "" {
		return errors.New("username cannot be empty")
	}

	if len(username) > 30 {
		return errors.New("username cannot exceed 30 characters")
	}

	validChars := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_$"
	for _, c := range username {
		if !strings.ContainsRune(validChars, c) {
			return fmt.Errorf("username contains invalid character '%c'", c)
		}
	}

	upperUsername := strings.ToUpper(username)

	for _, pattern := range sqlInjectionPatterns {
		if strings.Contains(upperUsername, strings.ToUpper(pattern)) {
			return fmt.Errorf("%w: found pattern '%s'", ErrDangerousPattern, pattern)
		}
	}

	dangerousUsernameKeywords := []string{
		"SELECT", "INSERT", "UPDATE", "DELETE", "DROP", "CREATE", "ALTER", "TRUNCATE",
		"GRANT", "REVOKE", "EXEC", "EXECUTE", "CALL", "DECLARE", "MERGE",
		"UNION", "INTO", "FROM", "WHERE", "JOIN", "GROUP BY", "ORDER BY",
	}

	for _, keyword := range dangerousUsernameKeywords {
		if strings.Contains(upperUsername, keyword) {
			return fmt.Errorf("%w: found SQL keyword '%s'", ErrDangerousPattern, keyword)
		}
	}

	return nil
}

func ValidateStatementTemplates(creation, revocation, rollback, renewal string) error {
	if creation != "" {
		if err := ValidateSQLStatement(creation); err != nil {
			return fmt.Errorf("creation_statement validation failed: %w", err)
		}
	}

	if revocation != "" {
		if err := ValidateSQLStatement(revocation); err != nil {
			return fmt.Errorf("revocation_statement validation failed: %w", err)
		}
	}

	if rollback != "" {
		if err := ValidateSQLStatement(rollback); err != nil {
			return fmt.Errorf("rollback_statement validation failed: %w", err)
		}
	}

	if renewal != "" {
		if err := ValidateSQLStatement(renewal); err != nil {
			return fmt.Errorf("renewal_statement validation failed: %w", err)
		}
	}

	return nil
}

func ValidateSessionVariables(vars map[string]string) error {
	if len(vars) > MaxSessionVars {
		return fmt.Errorf("%w: maximum %d variables allowed", ErrInvalidSessionVar, MaxSessionVars)
	}

	validNameChars := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_"
	dangerousPatterns := []string{"--", "/*", "*/", ";", "'", "\"", "\\", "\x00", "0x", "char(", "nchar(", "varchar("}

	for name, value := range vars {
		if len(name) > MaxSessionVarNameLen {
			return fmt.Errorf("%w: name '%s' exceeds maximum length %d", ErrSessionVarTooLong, name, MaxSessionVarNameLen)
		}

		if len(value) > MaxSessionVarValueLen {
			return fmt.Errorf("%w: value for '%s' exceeds maximum length %d", ErrSessionVarTooLong, name, MaxSessionVarValueLen)
		}

		for _, c := range name {
			if !strings.ContainsRune(validNameChars, c) {
				return fmt.Errorf("%w: name '%s' contains invalid character '%c'", ErrInvalidSessionVar, name, c)
			}
		}

		upperValue := strings.ToUpper(value)
		for _, pattern := range dangerousPatterns {
			if strings.Contains(upperValue, pattern) {
				return fmt.Errorf("%w: value for '%s' contains dangerous pattern '%s'", ErrDangerousPattern, name, pattern)
			}
		}

		dangerousKeywords := []string{"SELECT", "INSERT", "UPDATE", "DELETE", "DROP", "CREATE", "ALTER", "GRANT", "REVOKE", "EXEC", "EXECUTE"}
		for _, keyword := range dangerousKeywords {
			if strings.Contains(upperValue, keyword) {
				idx := strings.Index(upperValue, keyword)
				before := ""
				if idx > 0 {
					before = string(upperValue[idx-1])
				}
				after := ""
				if idx+len(keyword) < len(upperValue) {
					after = string(upperValue[idx+len(keyword)])
				}
				isWordBoundary := (before == "" || !unicode.IsLetter([]rune(before)[0])) &&
					(after == "" || (!unicode.IsLetter([]rune(after)[0]) && after != "_" && after != "("))
				if isWordBoundary {
					return fmt.Errorf("%w: value for '%s' contains SQL keyword '%s'", ErrDangerousPattern, name, keyword)
				}
			}
		}
	}

	return nil
}

func SanitizeStringForSQL(input string) string {
	var result strings.Builder
	for _, r := range input {
		switch r {
		case '\'':
			result.WriteString("''")
		case '"':
			result.WriteString("\"\"")
		case ';':
			result.WriteRune(r)
		case '-':
			result.WriteRune(r)
		case '/':
			result.WriteRune(r)
		case '*':
			result.WriteRune(r)
		case '\\':
			result.WriteString("\\\\")
		default:
			result.WriteRune(r)
		}
	}
	return result.String()
}

type StatementType int

const (
	StatementTypeUnknown StatementType = iota
	StatementTypeGRANT
	StatementTypeREVOKE
)
