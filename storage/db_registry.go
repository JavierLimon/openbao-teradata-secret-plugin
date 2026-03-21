package storage

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	_ "github.com/JavierLimon/openbao-teradata-secret-plugin/odbc"
)

type DBConfig struct {
	Name                string        `json:"name"`
	ConnectionString    string        `json:"connection_string"`
	MaxOpenConnections  int           `json:"max_open_connections"`
	MaxIdleConnections  int           `json:"max_idle_connections"`
	ConnectionTimeout   time.Duration `json:"connection_timeout"`
	HealthCheckInterval time.Duration `json:"health_check_interval"`
	HealthCheckTimeout  time.Duration `json:"health_check_timeout"`
}

type ConnectionState int

const (
	StateUnknown ConnectionState = iota
	StateHealthy
	StateUnhealthy
	StateClosed
)

type DBConnection struct {
	Config          *DBConfig
	Database        *sql.DB
	mu              sync.RWMutex
	state           ConnectionState
	lastHealthCheck time.Time
	healthCheckErr  error
}

type DBRegistry struct {
	connections map[string]*DBConnection
	mu          sync.RWMutex
	ctx         context.Context
	cancel      context.CancelFunc
	healthDone  chan struct{}
}

func NewDBRegistry() *DBRegistry {
	ctx, cancel := context.WithCancel(context.Background())
	return &DBRegistry{
		connections: make(map[string]*DBConnection),
		ctx:         ctx,
		cancel:      cancel,
		healthDone:  make(chan struct{}),
	}
}

func (r *DBRegistry) StartHealthChecks() {
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-r.ctx.Done():
				close(r.healthDone)
				return
			case <-ticker.C:
				r.runHealthChecks()
			}
		}
	}()
}

func (r *DBRegistry) StopHealthChecks() {
	r.cancel()
	<-r.healthDone
}

func (r *DBRegistry) runHealthChecks() {
	r.mu.RLock()
	var connections []*DBConnection
	for _, conn := range r.connections {
		connections = append(connections, conn)
	}
	r.mu.RUnlock()

	for _, conn := range connections {
		conn.CheckHealth()
	}
}

func (c *DBConnection) CheckHealth() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.Database == nil {
		c.state = StateClosed
		c.healthCheckErr = fmt.Errorf("database connection is nil")
		return c.healthCheckErr
	}

	ctx, cancel := context.WithTimeout(context.Background(), c.Config.HealthCheckTimeout)
	defer cancel()

	err := c.Database.PingContext(ctx)
	if err != nil {
		c.state = StateUnhealthy
		c.healthCheckErr = fmt.Errorf("health check failed: %w", err)
	} else {
		c.state = StateHealthy
		c.healthCheckErr = nil
	}
	c.lastHealthCheck = time.Now()
	return c.healthCheckErr
}

func (c *DBConnection) GetState() (ConnectionState, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.state, c.healthCheckErr
}

func (c *DBConnection) LastHealthCheck() time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.lastHealthCheck
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

	if config.HealthCheckInterval == 0 {
		config.HealthCheckInterval = 30 * time.Second
	}
	if config.HealthCheckTimeout == 0 {
		config.HealthCheckTimeout = 5 * time.Second
	}
	if config.ConnectionTimeout == 0 {
		config.ConnectionTimeout = 10 * time.Second
	}

	db, err := sql.Open("odbc", config.ConnectionString)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	db.SetMaxOpenConns(config.MaxOpenConnections)
	db.SetMaxIdleConns(config.MaxIdleConnections)

	dbc := &DBConnection{
		Config:          config,
		Database:        db,
		state:           StateUnknown,
		lastHealthCheck: time.Now(),
	}

	r.connections[name] = dbc
	return dbc, nil
}

func (r *DBRegistry) RemoveConnection(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if conn, exists := r.connections[name]; exists && conn.Database != nil {
		conn.mu.Lock()
		conn.state = StateClosed
		conn.Database.Close()
		conn.mu.Unlock()
	}
	delete(r.connections, name)
}

func (r *DBRegistry) ListConnections() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.connections))
	for name := range r.connections {
		names = append(names, name)
	}
	return names
}

func (r *DBRegistry) GetConnectionStats(name string) (state ConnectionState, openConns int, idleConns int, err error) {
	r.mu.RLock()
	conn, ok := r.connections[name]
	r.mu.RUnlock()

	if !ok {
		return StateUnknown, 0, 0, fmt.Errorf("connection not found: %s", name)
	}

	conn.mu.RLock()
	state = conn.state
	err = conn.healthCheckErr
	conn.mu.RUnlock()

	openConns = conn.Database.Stats().OpenConnections
	idleConns = conn.Database.Stats().Idle

	return state, openConns, idleConns, err
}
