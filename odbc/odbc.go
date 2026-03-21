// Package odbc provides Teradata ODBC connectivity
package odbc

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

var (
	ErrNotConnected    = errors.New("not connected")
	ErrEmptyUsername   = errors.New("username cannot be empty")
	ErrInvalidUsername = errors.New("username contains invalid characters")
	ErrUsernameTooLong = errors.New("username cannot exceed 30 characters")
	ErrSQLInjection    = errors.New("potential SQL injection attempt detected")
)

var sqlKeywords = []string{
	"SELECT", "INSERT", "UPDATE", "DELETE", "DROP", "CREATE", "ALTER", "TRUNCATE",
	"GRANT", "REVOKE", "EXEC", "EXECUTE", "CALL", "DECLARE", "MERGE",
	"UNION", "INTO", "FROM", "WHERE", "JOIN", "GROUP BY", "ORDER BY",
}

var sqlInjectionPatterns = []string{
	";", "--", "/*", "*/", "xp_", "sp_", "sys.", "sysobjects",
	"waitfor", "delay", "0x", "char(", "nchar(", "varchar(",
}

type Connection struct {
	connString string
	connected  bool
	db         *sql.DB
}

func Connect(connString string) (*Connection, error) {
	db, err := sql.Open("odbc", connString)
	if err != nil {
		return nil, fmt.Errorf("failed to open ODBC connection: %w", err)
	}

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping ODBC connection: %w", err)
	}

	return &Connection{
		connString: connString,
		connected:  true,
		db:         db,
	}, nil
}

func (c *Connection) Close() error {
	if !c.connected || c.db == nil {
		return ErrNotConnected
	}
	if c.db != nil {
		c.db.Close()
	}
	c.connected = false
	return c.db.Close()
}

func (c *Connection) DB() *sql.DB {
	return c.db
}

func (c *Connection) Ping() error {
	if !c.connected || c.db == nil {
		return ErrNotConnected
	}
	return c.db.Ping()
}

func (c *Connection) Execute(query string, args ...interface{}) (sql.Result, error) {
	if !c.connected || c.db == nil {
		return nil, ErrNotConnected
	}
	return c.db.Exec(query, args...)
}

func (c *Connection) Query(query string, args ...interface{}) (*sql.Rows, error) {
	if !c.connected || c.db == nil {
		return nil, ErrNotConnected
	}
	return c.db.Query(query, args...)
}

func (c *Connection) QueryRow(query string, args ...interface{}) *sql.Row {
	if !c.connected || c.db == nil {
		return nil
	}
	return c.db.QueryRow(query, args...)
}

func CreateUser(db *sql.DB, username, password, defaultDB string, permSpace int64) error {
	query := fmt.Sprintf(
		"CREATE USER %s FROM DBC AS PASSWORD = %s DEFAULT DATABASE = %s PERM = %d",
		username,
		password,
		defaultDB,
		permSpace,
	)
	_, err := db.Exec(query)
	if err != nil {
		return fmt.Errorf("failed to create user %s: %w", username, err)
	}
	return nil
}

func DropUser(db *sql.DB, username string) error {
	query := fmt.Sprintf("DROP USER %s", username)
	_, err := db.Exec(query)
	if err != nil {
		return fmt.Errorf("failed to drop user %s: %w", username, err)
	}
	return nil
}

func GrantPrivileges(db *sql.DB, username, database string, privileges []string) error {
	for _, priv := range privileges {
		query := fmt.Sprintf("GRANT %s ON %s TO %s", priv, database, username)
		_, err := db.Exec(query)
		if err != nil {
			return fmt.Errorf("failed to grant %s on %s to %s: %w", priv, database, username, err)
		}
	}
	return nil
}

func RevokePrivileges(db *sql.DB, username, database string, privileges []string) error {
	for _, priv := range privileges {
		query := fmt.Sprintf("REVOKE %s ON %s FROM %s", priv, database, username)
		_, err := db.Exec(query)
		if err != nil {
			return fmt.Errorf("failed to revoke %s on %s from %s: %w", priv, database, username, err)
		}
	}
	return nil
}

func AlterUserPassword(db *sql.DB, username, newPassword string) error {
	query := fmt.Sprintf("MODIFY USER %s AS PASSWORD = %s", username, newPassword)
	_, err := db.Exec(query)
	if err != nil {
		return fmt.Errorf("failed to alter user password for %s: %w", username, err)
	}
	return nil
}

func ValidateUsername(username string) error {
	if username == "" {
		return ErrEmptyUsername
	}
	if len(username) > 30 {
		return ErrUsernameTooLong
	}

	upperUsername := strings.ToUpper(username)

	for _, pattern := range sqlInjectionPatterns {
		if strings.Contains(upperUsername, strings.ToUpper(pattern)) {
			return fmt.Errorf("%w: found pattern '%s'", ErrSQLInjection, pattern)
		}
	}

	for _, keyword := range sqlKeywords {
		if strings.Contains(upperUsername, keyword) {
			return fmt.Errorf("%w: found SQL keyword '%s'", ErrSQLInjection, keyword)
		}
	}

	validChars := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_$"
	for _, c := range username {
		if !strings.ContainsRune(validChars, c) {
			return fmt.Errorf("%w: invalid character '%c'", ErrInvalidUsername, c)
		}
	}
	return nil
}

// ExecuteGrantStatements executes multiple GRANT statements
// Statements are separated by semicolons. Empty statements are skipped.
func (c *Connection) ExecuteGrantStatements(grantStatements string) error {
	if !c.connected {
		return errors.New("not connected")
	}

	if strings.TrimSpace(grantStatements) == "" {
		return nil
	}

	statements := parseSQLStatements(grantStatements)

	for _, stmt := range statements {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}

		stmt = normalizeGrantStatement(stmt)
		if stmt == "" {
			continue
		}

		_, err := c.db.Exec(stmt)
		if err != nil {
			return err
		}
	}

	return nil
}

// parseSQLStatements splits a multi-statement SQL string into individual statements
// It handles semicolon-separated statements
func parseSQLStatements(sql string) []string {
	var statements []string
	var current strings.Builder

	lines := strings.Split(sql, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if trimmed == "" {
			continue
		}

		current.WriteString(line)
		current.WriteString("\n")

		if strings.HasSuffix(trimmed, ";") {
			statements = append(statements, current.String())
			current.Reset()
		}
	}

	if current.Len() > 0 {
		statements = append(statements, current.String())
	}

	return statements
}

// normalizeGrantStatement normalizes a GRANT statement
// Returns empty string if the statement is not a GRANT statement
func normalizeGrantStatement(stmt string) string {
	stmt = strings.TrimSpace(stmt)

	upperStmt := strings.ToUpper(stmt)
	upperStmt = strings.TrimSpace(upperStmt)

	if !strings.HasPrefix(upperStmt, "GRANT") {
		return ""
	}

	return stmt
}

// ExecuteMultipleStatements executes multiple SQL statements separated by semicolons
// Returns error if any statement fails
func (c *Connection) ExecuteMultipleStatements(sqlStatements string) error {
	if !c.connected {
		return errors.New("not connected")
	}

	if strings.TrimSpace(sqlStatements) == "" {
		return nil
	}

	statements := parseSQLStatements(sqlStatements)

	for _, stmt := range statements {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}

		stmt = strings.TrimSuffix(stmt, ";")
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}

		_, err := c.db.Exec(stmt)
		if err != nil {
			return err
		}
	}

	return nil
}
