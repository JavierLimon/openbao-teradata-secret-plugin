package storage

import (
	"database/sql"
	"fmt"
	"sync"

	_ "github.com/JavierLimon/openbao-teradata-secret-plugin/odbc"
)

type DBConfig struct {
	Name               string `json:"name"`
	ConnectionString   string `json:"connection_string"`
	MaxOpenConnections int    `json:"max_open_connections"`
	MaxIdleConnections int    `json:"max_idle_connections"`
	ConnectionTimeout  int    `json:"connection_timeout"`
}

type DBConnection struct {
	Config   *DBConfig
	Database *sql.DB
	mu       sync.RWMutex
}

type DBRegistry struct {
	connections map[string]*DBConnection
	mu          sync.RWMutex
}

func NewDBRegistry() *DBRegistry {
	return &DBRegistry{
		connections: make(map[string]*DBConnection),
	}
}

func (r *DBRegistry) GetConnection(name string) (*DBConnection, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	conn, ok := r.connections[name]
	return conn, ok
}

func (r *DBRegistry) AddConnection(name string, config *DBConfig) (*DBConnection, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if conn, exists := r.connections[name]; exists {
		return conn, nil
	}

	db, err := sql.Open("odbc", config.ConnectionString)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if config.MaxOpenConnections > 0 {
		db.SetMaxOpenConns(config.MaxOpenConnections)
	}
	if config.MaxIdleConnections > 0 {
		db.SetMaxIdleConns(config.MaxIdleConnections)
	}

	dbc := &DBConnection{
		Config:   config,
		Database: db,
	}

	r.connections[name] = dbc
	return dbc, nil
}

func (r *DBRegistry) RemoveConnection(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if conn, exists := r.connections[name]; exists && conn.Database != nil {
		conn.Database.Close()
	}
	delete(r.connections, name)
}
