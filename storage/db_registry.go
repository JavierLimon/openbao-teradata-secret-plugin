package storage

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/JavierLimon/openbao-teradata-secret-plugin/logging"
	"github.com/JavierLimon/openbao-teradata-secret-plugin/metrics"
	_ "github.com/JavierLimon/openbao-teradata-secret-plugin/odbc"
	"github.com/JavierLimon/openbao-teradata-secret-plugin/retry"
)

type DBConfig struct {
	Name                  string        `json:"name"`
	ConnectionString      string        `json:"connection_string"`
	MinConnections        int           `json:"min_connections"`
	MaxOpenConnections    int           `json:"max_open_connections"`
	MaxIdleConnections    int           `json:"max_idle_connections"`
	ConnectionTimeout     time.Duration `json:"connection_timeout"`
	MaxConnectionLifetime time.Duration `json:"max_connection_lifetime"`
	IdleTimeout           time.Duration `json:"idle_timeout"`
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
	MaxRetries            int           `json:"max_retries"`
	InitialRetryInterval  time.Duration `json:"initial_retry_interval"`
	MaxRetryInterval      time.Duration `json:"max_retry_interval"`
	RetryMultiplier       float64       `json:"retry_multiplier"`
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
	connections    map[string]*DBConnection
	mu             sync.RWMutex
	healthCtx      context.Context
	healthCancel   context.CancelFunc
	healthDone     chan struct{}
	cleanupCtx     context.Context
	cleanupCancel  context.CancelFunc
	cleanupDone    chan struct{}
	minConnsCtx    context.Context
	minConnsCancel context.CancelFunc
	minConnsDone   chan struct{}
}

type PoolStats struct {
	State             ConnectionState `json:"state"`
	OpenConnections   int             `json:"open_connections"`
	InUse             int             `json:"in_use"`
	Idle              int             `json:"idle"`
	MaxOpen           int             `json:"max_open"`
	MinConnections    int             `json:"min_connections"`
	WaitCount         int64           `json:"wait_count"`
	WaitDurationNanos int64           `json:"wait_duration_nanos"`
	MaxIdleClosed     int64           `json:"max_idle_closed"`
	MaxLifetimeClosed int64           `json:"max_lifetime_closed"`
	LastHealthCheck   time.Time       `json:"last_health_check"`
	HealthError       error           `json:"health_error,omitempty"`
}

func NewDBRegistry() *DBRegistry {
	healthCtx, healthCancel := context.WithCancel(context.Background())
	cleanupCtx, cleanupCancel := context.WithCancel(context.Background())
	minConnsCtx, minConnsCancel := context.WithCancel(context.Background())
	return &DBRegistry{
		connections:    make(map[string]*DBConnection),
		healthCtx:      healthCtx,
		healthCancel:   healthCancel,
		healthDone:     make(chan struct{}),
		cleanupCtx:     cleanupCtx,
		cleanupCancel:  cleanupCancel,
		cleanupDone:    make(chan struct{}),
		minConnsCtx:    minConnsCtx,
		minConnsCancel: minConnsCancel,
		minConnsDone:   make(chan struct{}),
	}
}

func (r *DBRegistry) StartHealthChecks() {
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-r.healthCtx.Done():
				close(r.healthDone)
				return
			case <-ticker.C:
				r.runHealthChecks()
			}
		}
	}()
}

func (r *DBRegistry) StopHealthChecks() {
	r.healthCancel()
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
			case <-r.cleanupCtx.Done():
				close(r.cleanupDone)
				return
			case <-ticker.C:
				r.runCleanup()
			}
		}
	}()
}

func (r *DBRegistry) StopCleanupJob() {
	r.cleanupCancel()
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

	if c.Database == nil || c.Config.IdleTimeout <= 0 {
		return
	}

	if time.Since(c.lastUsed) > c.Config.IdleTimeout {
		c.Database.Close()
		c.state = StateClosed
		metrics.PoolIdleClosedTotal.WithLabelValues(c.Config.Name).Inc()
		stats := c.Database.Stats()
		metrics.PoolOpenConnections.WithLabelValues(c.Config.Name).Set(float64(stats.OpenConnections))
		metrics.PoolIdleConnections.WithLabelValues(c.Config.Name).Set(float64(stats.Idle))
		logging.LogConnectionEvent(nil, "idle_connection_closed", c.Config.Name, map[string]interface{}{
			"idle_timeout": c.Config.IdleTimeout,
			"last_used":    c.lastUsed,
		})
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

	startTime := time.Now()

	if c.Database == nil {
		c.state = StateClosed
		c.healthCheckErr = fmt.Errorf("database connection is nil")
		metrics.PoolHealthCheckDuration.WithLabelValues(c.Config.Name, "closed").Observe(time.Since(startTime).Seconds())
		metrics.PoolHealthCheckTotal.WithLabelValues(c.Config.Name, "closed").Inc()
		metrics.PoolConnectionErrors.WithLabelValues(c.Config.Name, "nil_connection").Inc()
		logging.LogHealthCheck(nil, c.Config.Name, "closed", time.Since(startTime), c.healthCheckErr)
		return c.healthCheckErr
	}

	ctx, cancel := context.WithTimeout(context.Background(), c.Config.HealthCheckTimeout)
	defer cancel()

	err := c.Database.PingContext(ctx)
	duration := time.Since(startTime).Seconds()
	if err != nil {
		c.state = StateUnhealthy
		c.healthCheckErr = fmt.Errorf("health check failed: %w", err)
		metrics.PoolHealthCheckDuration.WithLabelValues(c.Config.Name, "unhealthy").Observe(duration)
		metrics.PoolHealthCheckTotal.WithLabelValues(c.Config.Name, "unhealthy").Inc()
		metrics.PoolConnectionErrors.WithLabelValues(c.Config.Name, "ping_failed").Inc()
		logging.LogHealthCheck(nil, c.Config.Name, "unhealthy", time.Since(startTime), c.healthCheckErr)
	} else {
		c.state = StateHealthy
		c.healthCheckErr = nil
		metrics.PoolHealthCheckDuration.WithLabelValues(c.Config.Name, "healthy").Observe(duration)
		metrics.PoolHealthCheckTotal.WithLabelValues(c.Config.Name, "healthy").Inc()
		logging.LogHealthCheck(nil, c.Config.Name, "healthy", time.Since(startTime), nil)
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
	if config.IdleTimeout == 0 {
		config.IdleTimeout = 5 * time.Minute
	}
	if config.MaxConnectionLifetime == 0 {
		config.MaxConnectionLifetime = 1 * time.Hour
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

	if config.MaxRetries == 0 {
		config.MaxRetries = 3
	}
	if config.InitialRetryInterval == 0 {
		config.InitialRetryInterval = 100 * time.Millisecond
	}
	if config.MaxRetryInterval == 0 {
		config.MaxRetryInterval = 5 * time.Second
	}
	if config.RetryMultiplier == 0 {
		config.RetryMultiplier = 2.0
	}

	db, err := sql.Open("odbc", config.ConnectionString)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	db.SetMaxOpenConns(config.MaxOpenConnections)
	db.SetMaxIdleConns(config.MaxIdleConnections)
	db.SetConnMaxLifetime(config.MaxConnectionLifetime)

	retryCfg := &retry.Config{
		MaxAttempts:     config.MaxRetries,
		InitialInterval: config.InitialRetryInterval,
		MaxInterval:     config.MaxRetryInterval,
		Multiplier:      config.RetryMultiplier,
	}

	ctx, cancel := context.WithTimeout(context.Background(), config.ConnectionTimeout)
	defer cancel()

	var pingErr error
	err = retry.Do(ctx, retryCfg, func() error {
		pingErr = db.PingContext(ctx)
		return pingErr
	})
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to connect to database after %d attempts: %w", config.MaxRetries, pingErr)
	}

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

	metrics.PoolOpenConnections.WithLabelValues(name).Set(float64(config.MaxOpenConnections))
	metrics.PoolIdleConnections.WithLabelValues(name).Set(0)
	metrics.PoolActiveConnections.WithLabelValues(name).Set(0)

	logging.LogConnectionEvent(nil, "connection_added", name, map[string]interface{}{
		"min_connections":         config.MinConnections,
		"max_open_connections":    config.MaxOpenConnections,
		"max_idle_connections":    config.MaxIdleConnections,
		"max_connection_lifetime": config.MaxConnectionLifetime,
		"idle_timeout":            config.IdleTimeout,
		"health_check_interval":   config.HealthCheckInterval,
		"max_retries":             config.MaxRetries,
		"initial_retry_interval":  config.InitialRetryInterval,
		"max_retry_interval":      config.MaxRetryInterval,
		"retry_multiplier":        config.RetryMultiplier,
	})

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
	if r.minConnsCtx.Err() != nil {
		return
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		r.EnsureMinConnections()
		for {
			select {
			case <-r.minConnsCtx.Done():
				close(r.minConnsDone)
				return
			case <-ticker.C:
				r.EnsureMinConnections()
			}
		}
	}()
}

func (r *DBRegistry) StopMinConnectionsJob() {
	r.minConnsCancel()
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
		metrics.PoolOpenConnections.WithLabelValues(name).Set(0)
		metrics.PoolIdleConnections.WithLabelValues(name).Set(0)
		metrics.PoolActiveConnections.WithLabelValues(name).Set(0)
		logging.LogConnectionEvent(nil, "connection_removed", name, nil)
	}
	delete(r.connections, name)
}

func (r *DBRegistry) UpdateConnection(name string, config *DBConfig) (*DBConnection, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	existingConn, exists := r.connections[name]
	if exists {
		if existingConn.Database != nil {
			existingConn.mu.Lock()
			existingConn.state = StateClosed
			existingConn.Database.Close()
			existingConn.mu.Unlock()
		}
		delete(r.connections, name)
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
	if config.IdleTimeout == 0 {
		config.IdleTimeout = 5 * time.Minute
	}
	if config.MaxConnectionLifetime == 0 {
		config.MaxConnectionLifetime = 1 * time.Hour
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
	db.SetConnMaxLifetime(config.MaxConnectionLifetime)

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

	metrics.PoolOpenConnections.WithLabelValues(name).Set(float64(config.MaxOpenConnections))
	metrics.PoolIdleConnections.WithLabelValues(name).Set(0)
	metrics.PoolActiveConnections.WithLabelValues(name).Set(0)

	logging.LogConnectionEvent(nil, "connection_updated", name, map[string]interface{}{
		"min_connections":         config.MinConnections,
		"max_open_connections":    config.MaxOpenConnections,
		"max_idle_connections":    config.MaxIdleConnections,
		"max_connection_lifetime": config.MaxConnectionLifetime,
		"idle_timeout":            config.IdleTimeout,
		"health_check_interval":   config.HealthCheckInterval,
	})

	if config.MinConnections > 0 {
		go dbc.warmupPool()
	}

	return dbc, nil
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

func (r *DBRegistry) UpdatePoolMetrics() {
	r.mu.RLock()
	var connections []*DBConnection
	for _, conn := range r.connections {
		connections = append(connections, conn)
	}
	r.mu.RUnlock()

	for _, conn := range connections {
		conn.updateMetrics()
	}
}

func (c *DBConnection) updateMetrics() {
	if c.Database == nil {
		return
	}

	stats := c.Database.Stats()
	metrics.PoolOpenConnections.WithLabelValues(c.Config.Name).Set(float64(stats.OpenConnections))
	metrics.PoolIdleConnections.WithLabelValues(c.Config.Name).Set(float64(stats.Idle))
	metrics.PoolActiveConnections.WithLabelValues(c.Config.Name).Set(float64(stats.OpenConnections - stats.Idle))
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

	metrics.PoolOpenConnections.WithLabelValues(name).Set(float64(openConns))
	metrics.PoolIdleConnections.WithLabelValues(name).Set(float64(idleConns))
	metrics.PoolActiveConnections.WithLabelValues(name).Set(float64(openConns - idleConns))

	return state, openConns, idleConns, err
}

func (r *DBRegistry) GetDetailedConnectionStats(name string) (*PoolStats, error) {
	r.mu.RLock()
	conn, ok := r.connections[name]
	r.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("connection not found: %s", name)
	}

	conn.mu.RLock()
	state := conn.state
	healthErr := conn.healthCheckErr
	lastHealthCheck := conn.lastHealthCheck
	conn.mu.RUnlock()

	stats := conn.Database.Stats()

	return &PoolStats{
		State:             state,
		OpenConnections:   stats.OpenConnections,
		InUse:             stats.InUse,
		Idle:              stats.Idle,
		MaxOpen:           stats.MaxOpenConnections,
		MinConnections:    conn.Config.MinConnections,
		WaitCount:         stats.WaitCount,
		WaitDurationNanos: stats.WaitDuration.Nanoseconds(),
		MaxIdleClosed:     stats.MaxIdleClosed,
		MaxLifetimeClosed: stats.MaxLifetimeClosed,
		LastHealthCheck:   lastHealthCheck,
		HealthError:       healthErr,
	}, nil
}

func (r *DBRegistry) Shutdown() {
	r.healthCancel()
	r.cleanupCancel()
	r.minConnsCancel()

	<-r.healthDone
	<-r.cleanupDone
	<-r.minConnsDone

	r.mu.Lock()
	defer r.mu.Unlock()

	for name, conn := range r.connections {
		if conn != nil && conn.Database != nil {
			conn.mu.Lock()
			conn.state = StateClosed
			conn.Database.Close()
			conn.mu.Unlock()
		}
		delete(r.connections, name)
	}
}
