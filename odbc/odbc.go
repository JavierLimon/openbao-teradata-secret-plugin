// Package odbc provides Teradata ODBC connectivity
package odbc

import (
	"database/sql"
	"errors"
	"strings"
)

// Connection represents an ODBC connection
type Connection struct {
	connString string
	connected  bool
	db         *sql.DB
}

// Connect establishes an ODBC connection
func Connect(connString string) (*Connection, error) {
	db, err := sql.Open("odbc", connString)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}

	return &Connection{
		connString: connString,
		connected:  true,
		db:         db,
	}, nil
}

// Close closes the ODBC connection
func (c *Connection) Close() error {
	if !c.connected {
		return errors.New("not connected")
	}
	if c.db != nil {
		c.db.Close()
	}
	c.connected = false
	return nil
}

// Ping tests the connection
func (c *Connection) Ping() error {
	if !c.connected {
		return errors.New("not connected")
	}
	return c.db.Ping()
}

// Execute runs a single SQL statement
func (c *Connection) Execute(sql string) error {
	if !c.connected {
		return errors.New("not connected")
	}

	_, err := c.db.Exec(sql)
	return err
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
