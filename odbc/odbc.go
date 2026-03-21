// Package odbc provides Teradata ODBC connectivity
package odbc

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	_ "github.com/alexbrainman/odbc"
)

var ErrNotConnected = errors.New("not connected")

type Connection struct {
	db        *sql.DB
	connected bool
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
		db:        db,
		connected: true,
	}, nil
}

func (c *Connection) Close() error {
	if !c.connected || c.db == nil {
		return ErrNotConnected
	}
	c.connected = false
	return c.db.Close()
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
		return errors.New("username cannot be empty")
	}
	if len(username) > 30 {
		return errors.New("username cannot exceed 30 characters")
	}
	validChars := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_$"
	for _, c := range username {
		if !strings.ContainsRune(validChars, c) {
			return fmt.Errorf("username contains invalid character: %c", c)
		}
	}
	return nil
}
