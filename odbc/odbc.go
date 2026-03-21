// Package odbc provides Teradata ODBC connectivity
package odbc

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"
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

var invalidConnectionStringChars = []string{
	";", "--", "/*", "*/", "'", "\"", "\\", "\x00",
}

var requiredConnectionParams = []string{
	"DSN",
	"SERVER",
}

type ConnectionStringConfig struct {
	DSN      string
	SERVER   string
	UID      string
	PWD      string
	DATABASE string
}

type ConnectionStringValidationError struct {
	Field   string
	Message string
}

func (e *ConnectionStringValidationError) Error() string {
	return fmt.Sprintf("connection string validation failed for field '%s': %s", e.Field, e.Message)
}

type Connection struct {
	connString      string
	connected       bool
	db              *sql.DB
	mu              sync.RWMutex
	lastValidated   time.Time
	keepAliveCtx    context.Context
	keepAliveCancel context.CancelFunc
	keepAliveDone   chan struct{}
	keepAliveInt    time.Duration
}

type SSLConfig struct {
	Mode         string
	Cert         string
	Key          string
	RootCert     string
	KeyPassword  string
	CipherSuites string
	Secure       bool
	Version      string
}

type TeradataConnectionConfig struct {
	DSN               string
	Server            string
	Servers           string
	Port              int
	Database          string
	Username          string
	Password          string
	ConnectionTimeout int
	QueryTimeout      int
	SessionMode       string
	Account           string
	SSLMode           string
	SSLCert           string
	SSLKey            string
	SSLRootCert       string
	SSLKeyPassword    string
	SSLCipherSuites   string
	SSLSecure         bool
	SSLVersion        string
}

func BuildTeradataConnectionString(cfg TeradataConnectionConfig) string {
	var params []string

	if cfg.DSN != "" {
		params = append(params, fmt.Sprintf("DSN=%s", cfg.DSN))
	}
	if cfg.Server != "" {
		params = append(params, fmt.Sprintf("SERVER=%s", cfg.Server))
	}
	if cfg.Servers != "" {
		params = append(params, fmt.Sprintf("SERVERS=%s", cfg.Servers))
	}
	if cfg.Port > 0 {
		params = append(params, fmt.Sprintf("PORT=%d", cfg.Port))
	}
	if cfg.Database != "" {
		params = append(params, fmt.Sprintf("DATABASE=%s", cfg.Database))
	}
	if cfg.Username != "" {
		params = append(params, fmt.Sprintf("UID=%s", cfg.Username))
	}
	if cfg.Password != "" {
		params = append(params, fmt.Sprintf("PWD=%s", cfg.Password))
	}
	if cfg.ConnectionTimeout > 0 {
		params = append(params, fmt.Sprintf("CONNTIMEOUT=%d", cfg.ConnectionTimeout))
	}
	if cfg.QueryTimeout > 0 {
		params = append(params, fmt.Sprintf("QUERYTIMEOUT=%d", cfg.QueryTimeout))
	}
	if cfg.SessionMode != "" {
		params = append(params, fmt.Sprintf("SESSIONMODE=%s", cfg.SessionMode))
	}
	if cfg.Account != "" {
		params = append(params, fmt.Sprintf("ACCOUNT=%s", cfg.Account))
	}
	if cfg.SSLMode != "" {
		params = append(params, fmt.Sprintf("SSLMODE=%s", cfg.SSLMode))
	}
	if cfg.SSLSecure {
		params = append(params, "SSL=1")
	}
	if cfg.SSLCert != "" {
		params = append(params, fmt.Sprintf("SSLCERT=%s", cfg.SSLCert))
	}
	if cfg.SSLKey != "" {
		params = append(params, fmt.Sprintf("SSLKEY=%s", cfg.SSLKey))
	}
	if cfg.SSLRootCert != "" {
		params = append(params, fmt.Sprintf("SSLROOTCERT=%s", cfg.SSLRootCert))
	}
	if cfg.SSLKeyPassword != "" {
		params = append(params, fmt.Sprintf("SSLKEYPASSWORD=%s", cfg.SSLKeyPassword))
	}
	if cfg.SSLCipherSuites != "" {
		params = append(params, fmt.Sprintf("SSLCIPHERSUITE=%s", cfg.SSLCipherSuites))
	}
	if cfg.SSLVersion != "" {
		params = append(params, fmt.Sprintf("SSLVERSION=%s", cfg.SSLVersion))
	}

	return strings.Join(params, ";")
}

func BuildConnectionString(baseConnString string, ssl *SSLConfig) string {
	if ssl == nil {
		return baseConnString
	}

	var sslParams []string

	if ssl.Mode != "" {
		sslParams = append(sslParams, fmt.Sprintf("SSLMODE=%s", ssl.Mode))
	}
	if ssl.Secure {
		sslParams = append(sslParams, "SSL=1")
	}
	if ssl.Cert != "" {
		sslParams = append(sslParams, fmt.Sprintf("SSLCERT=%s", ssl.Cert))
	}
	if ssl.Key != "" {
		sslParams = append(sslParams, fmt.Sprintf("SSLKEY=%s", ssl.Key))
	}
	if ssl.RootCert != "" {
		sslParams = append(sslParams, fmt.Sprintf("SSLROOTCERT=%s", ssl.RootCert))
	}
	if ssl.KeyPassword != "" {
		sslParams = append(sslParams, fmt.Sprintf("SSLKEYPASSWORD=%s", ssl.KeyPassword))
	}
	if ssl.CipherSuites != "" {
		sslParams = append(sslParams, fmt.Sprintf("SSLCIPHERSUITE=%s", ssl.CipherSuites))
	}
	if ssl.Version != "" {
		sslParams = append(sslParams, fmt.Sprintf("SSLVERSION=%s", ssl.Version))
	}

	if len(sslParams) == 0 {
		return baseConnString
	}

	if strings.TrimSpace(baseConnString) == "" {
		return strings.Join(sslParams, ";")
	}

	return baseConnString + ";" + strings.Join(sslParams, ";")
}

func AppendQueryTimeout(baseConnString string, queryTimeout int) string {
	if queryTimeout <= 0 {
		return baseConnString
	}
	if strings.TrimSpace(baseConnString) == "" {
		return fmt.Sprintf("QUERYTIMEOUT=%d", queryTimeout)
	}
	return baseConnString + fmt.Sprintf(";QUERYTIMEOUT=%d", queryTimeout)
}

var (
	ErrEmptyTemplate    = errors.New("connection string template cannot be empty")
	ErrMissingParameter = errors.New("missing required parameter in template")
	ErrInvalidParameter = errors.New("invalid parameter value")
)

func ValidateConnectionStringParameter(value string) error {
	if value == "" {
		return ErrInvalidParameter
	}
	upperValue := strings.ToUpper(value)
	for _, pattern := range sqlInjectionPatterns {
		if strings.Contains(upperValue, strings.ToUpper(pattern)) {
			return fmt.Errorf("%w: potentially dangerous pattern '%s'", ErrInvalidParameter, pattern)
		}
	}
	return nil
}

func BuildConnectionStringFromTemplate(template string, params map[string]string) (string, error) {
	if strings.TrimSpace(template) == "" {
		return "", ErrEmptyTemplate
	}

	if params == nil {
		params = make(map[string]string)
	}

	result := template

	for key, value := range params {
		if value == "" {
			continue
		}
		if err := ValidateConnectionStringParameter(value); err != nil {
			return "", fmt.Errorf("parameter %s: %w", key, err)
		}
		placeholder := fmt.Sprintf("{{%s}}", key)
		result = strings.ReplaceAll(result, placeholder, value)
	}

	re := regexp.MustCompile(`\{\{([^}]+)\}\}`)
	unresolved := re.FindAllString(result, -1)
	if len(unresolved) > 0 {
		return "", fmt.Errorf("%w: %s", ErrMissingParameter, strings.Join(unresolved, ", "))
	}

	return result, nil
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
	c.StopKeepAlive()

	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.connected || c.db == nil {
		return ErrNotConnected
	}

	err := c.db.Close()
	c.connected = false
	c.db = nil
	return err
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

func (c *Connection) Validate() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.connected || c.db == nil {
		return ErrNotConnected
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := c.db.PingContext(ctx); err != nil {
		c.connected = false
		return fmt.Errorf("connection validation failed: %w", err)
	}

	c.lastValidated = time.Now()
	return nil
}

func (c *Connection) SetKeepAliveInterval(interval time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.keepAliveInt = interval
}

func (c *Connection) StartKeepAlive(ctx context.Context) {
	c.mu.Lock()
	if c.keepAliveInt <= 0 {
		c.keepAliveInt = 30 * time.Second
	}
	if c.keepAliveCtx != nil && c.keepAliveCancel != nil {
		c.keepAliveCancel()
	}
	c.keepAliveCtx, c.keepAliveCancel = context.WithCancel(ctx)
	c.keepAliveDone = make(chan struct{})
	keepAliveInt := c.keepAliveInt
	c.mu.Unlock()

	go func() {
		defer close(c.keepAliveDone)
		ticker := time.NewTicker(keepAliveInt)
		defer ticker.Stop()

		for {
			select {
			case <-c.keepAliveCtx.Done():
				return
			case <-ticker.C:
				c.mu.RLock()
				if !c.connected || c.db == nil {
					c.mu.RUnlock()
					return
				}
				pingCtx, pingCancel := context.WithTimeout(c.keepAliveCtx, 5*time.Second)
				err := c.db.PingContext(pingCtx)
				pingCancel()
				c.lastValidated = time.Now()
				c.mu.RUnlock()

				if err != nil {
					return
				}
			}
		}
	}()
}

func (c *Connection) StopKeepAlive() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.keepAliveCancel != nil {
		c.keepAliveCancel()
	}
	if c.keepAliveDone != nil {
		<-c.keepAliveDone
	}
}

func (c *Connection) LastValidated() time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.lastValidated
}

func (c *Connection) IsHealthy() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected && c.db != nil
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

func ParseConnectionString(connString string) (*ConnectionStringConfig, error) {
	if strings.TrimSpace(connString) == "" {
		return nil, errors.New("connection string cannot be empty")
	}

	config := &ConnectionStringConfig{}
	parts := strings.Split(connString, ";")

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		idx := strings.Index(part, "=")
		if idx == -1 {
			return nil, fmt.Errorf("invalid connection string format: missing '=' in '%s'", part)
		}

		key := strings.ToUpper(strings.TrimSpace(part[:idx]))
		value := strings.TrimSpace(part[idx+1:])

		switch key {
		case "DSN":
			config.DSN = value
		case "SERVER":
			config.SERVER = value
		case "UID":
			config.UID = value
		case "PWD":
			config.PWD = value
		case "DATABASE":
			config.DATABASE = value
		}
	}

	return config, nil
}

func ValidateConnectionString(connString string) error {
	if strings.TrimSpace(connString) == "" {
		return errors.New("connection string cannot be empty")
	}

	for _, char := range invalidConnectionStringChars {
		if strings.Contains(connString, char) {
			return fmt.Errorf("connection string contains invalid character: '%s'", char)
		}
	}

	parsed, err := ParseConnectionString(connString)
	if err != nil {
		return err
	}

	hasDSN := parsed.DSN != ""
	hasServer := parsed.SERVER != ""

	if !hasDSN && !hasServer {
		return errors.New("connection string must contain either DSN or SERVER parameter")
	}

	if parsed.UID != "" {
		if err := ValidateUsername(parsed.UID); err != nil {
			return fmt.Errorf("invalid UID: %w", err)
		}
	}

	return nil
}

func ContainsSQLInjectionPattern(value string) bool {
	upperValue := strings.ToUpper(value)
	for _, pattern := range sqlInjectionPatterns {
		if strings.Contains(upperValue, pattern) {
			return true
		}
	}
	return false
}

func SanitizeConnectionStringParameter(value string) string {
	sanitized := strings.TrimSpace(value)
	for _, char := range invalidConnectionStringChars {
		sanitized = strings.ReplaceAll(sanitized, char, "")
	}
	return sanitized
}
