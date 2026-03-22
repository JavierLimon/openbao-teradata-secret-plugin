package teradata

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/JavierLimon/openbao-teradata-secret-plugin/audit"
	"github.com/JavierLimon/openbao-teradata-secret-plugin/logging"
	"github.com/JavierLimon/openbao-teradata-secret-plugin/models"
	"github.com/JavierLimon/openbao-teradata-secret-plugin/odbc"
	"github.com/JavierLimon/openbao-teradata-secret-plugin/retry"
	"github.com/JavierLimon/openbao-teradata-secret-plugin/storage"
	"github.com/JavierLimon/openbao-teradata-secret-plugin/tracing"
	"github.com/JavierLimon/openbao-teradata-secret-plugin/webhook"
	"github.com/openbao/openbao/sdk/v2/framework"
	"github.com/openbao/openbao/sdk/v2/logical"
)

const (
	OperationPrefix = "teradata"
)

type Backend struct {
	*framework.Backend

	storage                    logical.Storage
	mu                         sync.RWMutex
	dbRegistry                 *storage.DBRegistry
	credCache                  *credentialCache
	queryCache                 *queryResultCache
	rateLimiter                *RateLimiterMiddleware
	degradedSince              time.Time
	gracefulDegradation        bool
	manuallyEnabledDegradation bool
}

var DefaultRateLimitConfig = RateLimitConfig{
	RequestsPerSecond: 100,
	BurstSize:         50,
	CleanupInterval:   5 * time.Minute,
}

var _ logical.Backend = (*Backend)(nil)

func Factory(ctx context.Context, cfg *logical.BackendConfig) (logical.Backend, error) {
	b := NewBackend()
	if err := b.Setup(ctx, cfg); err != nil {
		return nil, err
	}
	return b, nil
}

func NewBackend() *Backend {
	return &Backend{}
}

func (b *Backend) Setup(ctx context.Context, cfg *logical.BackendConfig) error {
	b.Backend = &framework.Backend{
		Help: backendHelp,
		PathsSpecial: &logical.Paths{
			SealWrapStorage: []string{"config"},
		},
		Paths:       b.paths(),
		BackendType: logical.TypeLogical,
	}

	b.storage = cfg.StorageView
	b.dbRegistry = storage.NewDBRegistry()
	b.dbRegistry.StartHealthChecks()
	b.credCache = newCredentialCache(5*time.Minute, 10000)
	b.queryCache = newQueryResultCache(5*time.Minute, 1000)
	b.rateLimiter = NewRateLimiterMiddleware(b, DefaultRateLimitConfig, true)

	if err := b.prewarmPools(ctx); err != nil {
		return fmt.Errorf("failed to prewarm connection pools: %w", err)
	}

	return nil
}

func (b *Backend) prewarmPools(ctx context.Context) error {
	logging.Default().Info("pool_warmup_started",
		slog.String("component", logging.ComponentBackend),
		slog.String("operation", "prewarm_pools"),
	)
	configKeys := []string{"config"}

	entries, err := b.storage.List(ctx, "config/")
	if err != nil {
		return fmt.Errorf("failed to list config keys: %w", err)
	}
	for _, entry := range entries {
		configKeys = append(configKeys, "config/"+entry)
	}

	for _, key := range configKeys {
		entry, err := b.storage.Get(ctx, key)
		if err != nil {
			logging.LogError(logging.Default(), logging.ComponentBackend, "prewarm_config_load",
				fmt.Errorf("failed to load config key %s: %w", key, err),
				slog.String("config_key", key),
			)
			continue
		}
		if entry == nil {
			continue
		}

		var cfg models.Config
		if err := entry.DecodeJSON(&cfg); err != nil {
			logging.LogError(logging.Default(), logging.ComponentBackend, "prewarm_config_decode",
				fmt.Errorf("failed to decode config key %s: %w", key, err),
				slog.String("config_key", key),
			)
			continue
		}

		if cfg.ConnectionString == "" {
			continue
		}

		dbConfig := &storage.DBConfig{
			Name:                  cfg.Name,
			ConnectionString:      cfg.ConnectionString,
			MinConnections:        cfg.MinConnections,
			MaxOpenConnections:    cfg.MaxOpenConnections,
			MaxIdleConnections:    cfg.MaxIdleConnections,
			ConnectionTimeout:     time.Duration(cfg.ConnectionTimeout) * time.Second,
			MaxConnectionLifetime: time.Duration(cfg.MaxConnectionLifetime) * time.Second,
			IdleTimeout:           time.Duration(cfg.IdleTimeout) * time.Second,
			SSLMode:               cfg.SSLMode,
			SSLCert:               cfg.SSLCert,
			SSLKey:                cfg.SSLKey,
			SSLRootCert:           cfg.SSLRootCert,
			SSLKeyPassword:        cfg.SSLKeyPassword,
			SSLCipherSuites:       cfg.SSLCipherSuites,
			SSLSecure:             cfg.SSLSecure,
			SSLVersion:            cfg.SSLVersion,
		}

		name := cfg.Name
		if name == "" {
			name = "default"
		}

		conn, err := b.dbRegistry.AddConnection(name, dbConfig)
		if err != nil {
			logging.LogError(logging.Default(), logging.ComponentBackend, "prewarm_pool",
				fmt.Errorf("failed to add connection %s: %w", name, err),
				slog.String("connection_name", name),
			)
			continue
		}

		if conn != nil {
			conn.WaitForWarmup()
			logging.LogOperation(logging.Default(), logging.ComponentBackend, "pool_prewarmed",
				slog.String("connection_name", name),
				slog.Int("min_connections", dbConfig.MinConnections),
			)
		}
	}

	logging.Default().Info("pool_warmup_completed",
		slog.String("component", logging.ComponentBackend),
		slog.String("operation", "prewarm_pools"),
	)
	return nil
}

func (b *Backend) HandleRequest(ctx context.Context, req *logical.Request) (*logical.Response, error) {
	if req == nil {
		return b.Backend.HandleRequest(ctx, req)
	}

	ctx, span := tracing.StartSpan(ctx, "handle_request",
		tracing.WithAttributes(
			tracing.String("request.path", req.Path),
			tracing.String("request.operation", string(req.Operation)),
		),
	)
	defer func() {
		span.End()
	}()

	if err := b.rateLimiter.RateLimit(ctx, req); err != nil {
		span.RecordError(err)
		span.SetStatus(tracing.Error, err.Error())
		return nil, err
	}

	resp, err := b.Backend.HandleRequest(ctx, req)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(tracing.Error, err.Error())
	} else {
		span.SetStatus(tracing.Ok, "")
	}

	return resp, err
}

func (b *Backend) SetRateLimiterEnabled(enabled bool) {
	b.rateLimiter.SetEnabled(enabled)
}

func (b *Backend) IsRateLimiterEnabled() bool {
	return b.rateLimiter.IsEnabled()
}

func (b *Backend) GetRateLimitConfig() RateLimitConfig {
	return b.rateLimiter.GetConfig()
}

func (b *Backend) Shutdown() {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.dbRegistry != nil {
		b.dbRegistry.Shutdown()
	}
	if b.credCache != nil {
		b.credCache.Close()
	}
	if b.queryCache != nil {
		b.queryCache.Close()
	}
	if b.rateLimiter != nil && b.rateLimiter.limiter != nil {
		b.rateLimiter.limiter.Close()
	}
}

func (b *Backend) paths() []*framework.Path {
	return []*framework.Path{
		b.pathConfig(),
		b.pathConfigList(),
		b.pathConfigReset(),
		b.pathConfigReload(),
		b.pathConfigV1(),
		b.pathConfigBackup(),
		b.pathConfigRestore(),
		b.pathReloadConfig(),
		b.pathReloadConfigV1(),
		b.pathReloadPlugin(),
		b.pathWebhook(),
		b.pathRoles(),
		b.pathRolesV1(),
		b.pathRoleList(),
		b.pathRoleListV1(),
		b.pathStatements(),
		b.pathStatementList(),
		b.pathRotateRoot(),
		b.pathRotateRootV1(),
		b.pathCreds(),
		b.pathCredsV1(),
		b.pathCredsBatch(),
		b.pathRenewCreds(),
		b.pathRenewCredsBatch(),
		b.pathRevokeCreds(),
		b.pathBulkRevokeCreds(),
		b.pathBulkRevokeByRole(),
		b.pathRevokeAllCreds(),
		b.pathLeases(),
		b.pathLeaseInfo(),
		b.pathCleanupExpired(),
		b.pathLeaseLookup(),
		b.pathExtendLease(),
		b.pathStaticRoles(),
		b.pathStaticRoleList(),
		b.pathStaticCreds(),
		b.pathRotateStaticRole(),
		b.pathHealth(),
		b.pathReadiness(),
		b.pathLiveness(),
		b.pathDegradation(),
		b.pathVersion(),
		b.pathAPIVersion(),
		b.pathInfo(),
		b.pathPoolStats(),
		b.pathMetrics(),
		b.pathRateLimitConfig(),
		b.pathRateLimitStatus(),
	}
}

func (b *Backend) getDBRegistry() *storage.DBRegistry {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.dbRegistry
}

func (b *Backend) getCredCache() *credentialCache {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.credCache
}

func (b *Backend) getQueryCache() *queryResultCache {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.queryCache
}

func (b *Backend) IsDegraded() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.gracefulDegradation
}

func (b *Backend) DegradedSince() time.Time {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.degradedSince
}

func (b *Backend) SetGracefulDegradation(enabled bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if enabled && !b.gracefulDegradation {
		b.degradedSince = time.Now()
	} else if !enabled {
		b.degradedSince = time.Time{}
	}
	b.gracefulDegradation = enabled
}

func (b *Backend) IsGracefulDegradationManuallyEnabled() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.manuallyEnabledDegradation
}

func (b *Backend) SetManuallyEnabledDegradation(enabled bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.manuallyEnabledDegradation = enabled
	if enabled {
		b.gracefulDegradation = true
		if b.degradedSince.IsZero() {
			b.degradedSince = time.Now()
		}
	}
}

func (b *Backend) ShouldUseDegradation() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.gracefulDegradation || b.manuallyEnabledDegradation
}

func (b *Backend) IsPoolHealthy(region string) bool {
	registry := b.getDBRegistry()
	if registry == nil {
		return false
	}

	conn, ok := registry.GetConnection(region)
	if !ok {
		return false
	}

	state, err := conn.GetState()
	return err == nil && state == storage.StateHealthy
}

func (b *Backend) AreAllPoolsHealthy() bool {
	registry := b.getDBRegistry()
	if registry == nil {
		return false
	}

	connectionNames := registry.ListConnections()
	for _, name := range connectionNames {
		if !b.IsPoolHealthy(name) {
			return false
		}
	}
	return len(connectionNames) > 0
}

func (b *Backend) GetDegradationStatus() (isDegraded bool, degradedPools []string, healthyPools []string) {
	registry := b.getDBRegistry()
	if registry == nil {
		return true, nil, nil
	}

	connectionNames := registry.ListConnections()
	for _, name := range connectionNames {
		if b.IsPoolHealthy(name) {
			healthyPools = append(healthyPools, name)
		} else {
			degradedPools = append(degradedPools, name)
			isDegraded = true
		}
	}
	return
}

func (b *Backend) CanOperate(region string) (bool, string) {
	if b.AreAllPoolsHealthy() {
		return true, ""
	}

	degraded, degradedPools, healthyPools := b.GetDegradationStatus()
	if !degraded {
		return true, ""
	}

	if region != "" && b.IsPoolHealthy(region) {
		return true, ""
	}

	reason := "database pool is unavailable"
	if len(degradedPools) > 0 {
		reason = fmt.Sprintf("database pools unavailable: %v", degradedPools)
	}
	if len(healthyPools) > 0 {
		reason = fmt.Sprintf("region %q pool is unavailable; healthy pools: %v", region, healthyPools)
	}

	return false, reason
}

func (b *Backend) Revoke(ctx context.Context, leaseID string) error {
	if leaseID == "" {
		return nil
	}

	ctx, span := tracing.StartSpan(ctx, "revoke_credential")
	span.SetAttributes(
		tracing.String("lease_id", leaseID),
	)
	defer func() {
		span.End()
	}()

	parts := strings.Split(leaseID, "/")
	if len(parts) < 4 || parts[0] != "teradata" || parts[1] != "creds" {
		return nil
	}

	roleName := parts[2]
	username := parts[3]
	span.SetAttributes(
		tracing.String("role_name", roleName),
		tracing.String("username", username),
	)

	cred, err := b.getCachedCredential(ctx, b.storage, username)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to get credential for revocation: %w", err)
	}

	var cfg *models.Config
	if cred != nil && cred.Region != "" {
		cfg, err = getConfigByName(ctx, b.storage, cred.Region)
		if err != nil {
			span.RecordError(err)
			return fmt.Errorf("failed to get region config for revocation: %w", err)
		}
		if cfg == nil {
			return fmt.Errorf("configuration for region %q not found", cred.Region)
		}
	} else {
		cfg, err = getConfig(ctx, b.storage)
		if err != nil {
			span.RecordError(err)
			return fmt.Errorf("failed to get config for revocation: %w", err)
		}
		if cfg == nil {
			return fmt.Errorf("database configuration not found")
		}
	}

	role, err := getRole(ctx, b.storage, roleName)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to get role for revocation: %w", err)
	}

	var revokeSQL string
	if role != nil && role.RevocationStatement != "" {
		revokeSQL = strings.ReplaceAll(role.RevocationStatement, "{{username}}", username)
		var conn *odbc.Connection
		connString := odbc.AppendSessionTimeout(cfg.ConnectionString, cfg.SessionTimeout)
		connString = odbc.AppendQueryTimeout(connString, cfg.QueryTimeout)
		err = retry.Do(ctx, nil, func() error {
			conn, err = odbc.Connect(connString)
			return err
		})
		if err == nil {
			retry.Do(ctx, nil, func() error {
				return conn.ExecuteMultipleStatements(ctx, revokeSQL)
			})
			conn.Close()
		}
	}

	dropSQL := fmt.Sprintf("DROP USER %s", username)
	var conn *odbc.Connection
	connString := odbc.AppendSessionTimeout(cfg.ConnectionString, cfg.SessionTimeout)
	connString = odbc.AppendQueryTimeout(connString, cfg.QueryTimeout)
	err = retry.Do(ctx, nil, func() error {
		conn, err = odbc.Connect(connString)
		return err
	})
	if err != nil {
		span.RecordError(err)
		_ = audit.LogCredentialRevocation(ctx, b.storage, username, roleName, map[string]interface{}{"error": err.Error()})
		_ = webhook.SendCredentialRevokedWebhook(ctx, b.storage, username, roleName, map[string]interface{}{"error": err.Error()})
		return fmt.Errorf("failed to connect for revocation: %w", err)
	}
	defer conn.Close()

	err = retry.Do(ctx, nil, func() error {
		return conn.ExecuteMultipleStatements(ctx, dropSQL)
	})
	if err != nil {
		_ = audit.LogCredentialRevocation(ctx, b.storage, username, roleName, map[string]interface{}{"error": err.Error()})
		_ = webhook.SendCredentialRevokedWebhook(ctx, b.storage, username, roleName, map[string]interface{}{"error": err.Error()})
		return fmt.Errorf("failed to revoke credential: %w", err)
	}

	_ = audit.LogCredentialRevocation(ctx, b.storage, username, roleName, nil)
	_ = webhook.SendCredentialRevokedWebhook(ctx, b.storage, username, roleName, nil)

	b.invalidateCachedCredential(username)
	return nil
}

const backendHelp = `
The Teradata secrets engine provides dynamic database credentials
for Teradata databases using ODBC connectivity.

Once configured, roles can be created that define which database
user will be created and what permissions they should have.
`
