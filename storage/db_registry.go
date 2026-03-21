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
	Name                  string        `json:"name"`
	ConnectionString      string        `json:"connection_string"`
	MinConnections        int           `json:"min_connections"`
	MaxOpenConnections    int           `json:"max_open_connections"`
	MaxIdleConnections    int           `json:"max_idle_connections"`
	ConnectionTimeout     time.Duration `json:"connection_timeout"`
	IdleConnectionTimeout time.Duration `json:"idle_connection_timeout"`
	HealthCheckInterval   time.Duration `json:"health_check_interval"`
	HealthCheckTimeout    time.Duration `json:"health_check_timeout"`
	MinConnCheckInterval  time.Duration `json:"min_conn_check_interval"`
	SSLMode               string        `json:"ssl_mode"`
	SSLCert               string        `json:"ssl_cert"`
	SSLKey                string        `json:"ssl_key"`
	SSLRootCert           string        `json:"ssl_root_cert"`
	SSLKeyPassword        string        `json:"ssl_key_password"`
	SSLCipherSuites       string        `json:"ssl_cipher_suites"`
	SSLSecure             bool          `json:"ssl_secure"`
	SSLVersion            string        `json:"ssl_version"`
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
	lastUsed        time.Time
	healthCheckErr  error
	minConnections  int
	warmupDone      chan struct{}
}

type DBRegistry struct {
	connections  map[string]*DBConnection
	mu           sync.RWMutex
	ctx          context.Context
	cancel       context.CancelFunc
	healthDone   chan struct{}
	cleanupDone  chan struct{}
	minConnsDone chan struct{}
}

func NewDBRegistry() *DBRegistry {
	ctx, cancel := context.WithCancel(context.Background())
	return &DBRegistry{
		connections:  make(map[string]*DBConnection),
		ctx:          ctx,
		cancel:       cancel,
		healthDone:   make(chan struct{}),
		cleanupDone:  make(chan struct{}),
		minConnsDone: make(chan struct{}),
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

func (r *DBRegistry) StartCleanupJob(interval time.Duration) {
	if interval <= 0 {
		interval = 60 * time.Second
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-r.ctx.Done():
				close(r.cleanupDone)
				return
			case <-ticker.C:
				r.runCleanup()
			}
		}
	}()
}

func (r *DBRegistry) StopCleanupJob() {
	r.cancel()
	<-r.cleanupDone
}

func (r *DBRegistry) runCleanup() {
	r.mu.RLock()
	var connections []*DBConnection
	for _, conn := range r.connections {
		connections = append(connections, conn)
	}
	r.mu.RUnlock()

	for _, conn := range connections {
		conn.cleanupIdleConnections()
	}
}

func (c *DBConnection) cleanupIdleConnections() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.Database == nil || c.Config.IdleConnectionTimeout <= 0 {
		return
	}

	if time.Since(c.lastUsed) > c.Config.IdleConnectionTimeout {
		c.Database.Close()
		c.state = StateClosed
	}
}

func (c *DBConnection) TouchLastUsed() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastUsed = time.Now()
}

func (c *DBConnection) warmupPool() {
	defer close(c.warmupDone)

	if c.Database == nil || c.minConnections <= 0 {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), c.Config.ConnectionTimeout)
	defer cancel()

	for i := 0; i < c.minConnections; i++ {
		select {
		case <-ctx.Done():
			return
		default:
		}

		conn, err := c.Database.Conn(ctx)
		if err != nil {
			continue
		}
		conn.Close()
	}
}

func (c *DBConnection) WaitForWarmup() {
	if c.warmupDone != nil {
		<-c.warmupDone
	}
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
	if config.IdleConnectionTimeout == 0 {
		config.IdleConnectionTimeout = 5 * time.Minute
	}
	if config.MinConnections < 0 {
		config.MinConnections = 0
	}
	if config.MinConnections > config.MaxOpenConnections {
		config.MinConnections = config.MaxOpenConnections
	}
	if config.MaxIdleConnections > config.MaxOpenConnections {
		config.MaxIdleConnections = config.MaxOpenConnections
	}
	if config.MinConnections > config.MaxIdleConnections {
		config.MaxIdleConnections = config.MinConnections
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
		lastUsed:        time.Now(),
		minConnections:  config.MinConnections,
		warmupDone:      make(chan struct{}),
	}

	r.connections[name] = dbc

	if config.MinConnections > 0 {
		go dbc.warmupPool()
	}

	return dbc, nil
}

func (r *DBRegistry) EnsureMinConnections() error {
	r.mu.RLock()
	var connections []*DBConnection
	for _, conn := range r.connections {
		connections = append(connections, conn)
	}
	r.mu.RUnlock()

	for _, conn := range connections {
		if err := conn.ensureMinConnections(); err != nil {
			return err
		}
	}
	return nil
}

func (c *DBConnection) ensureMinConnections() error {
	if c.Config.MinConnections <= 0 {
		return nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.Database == nil {
		return fmt.Errorf("database connection is nil")
	}

	stats := c.Database.Stats()
	if stats.Idle >= c.Config.MinConnections {
		return nil
	}

	needed := c.Config.MinConnections - stats.Idle
	if stats.OpenConnections < c.Config.MaxOpenConnections {
		canOpen := c.Config.MaxOpenConnections - stats.OpenConnections
		toOpen := needed
		if toOpen > canOpen {
			toOpen = canOpen
		}

		for i := 0; i < toOpen; i++ {
			if err := c.pingAndTrackConnection(); err != nil {
				return err
			}
		}
	}

	return nil
}

func (c *DBConnection) pingAndTrackConnection() error {
	ctx, cancel := context.WithTimeout(context.Background(), c.Config.ConnectionTimeout)
	defer cancel()

	if err := c.Database.PingContext(ctx); err != nil {
		return fmt.Errorf("failed to open min connection: %w", err)
	}
	c.lastUsed = time.Now()
	return nil
}

func (r *DBRegistry) StartMinConnectionsJob() {
	interval := 30 * time.Second
	if r.ctx.Err() != nil {
		return
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		r.EnsureMinConnections()
		for {
			select {
			case <-r.ctx.Done():
				close(r.minConnsDone)
				return
			case <-ticker.C:
				r.EnsureMinConnections()
			}
		}
	}()
}

func (r *DBRegistry) StopMinConnectionsJob() {
	r.cancel()
	<-r.minConnsDone
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
