package teradata

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/JavierLimon/openbao-teradata-secret-plugin/audit"
	"github.com/JavierLimon/openbao-teradata-secret-plugin/models"
	"github.com/JavierLimon/openbao-teradata-secret-plugin/odbc"
	"github.com/JavierLimon/openbao-teradata-secret-plugin/retry"
	"github.com/JavierLimon/openbao-teradata-secret-plugin/storage"
	"github.com/JavierLimon/openbao-teradata-secret-plugin/webhook"
	"github.com/openbao/openbao/sdk/v2/framework"
	"github.com/openbao/openbao/sdk/v2/logical"
)

const (
	OperationPrefix = "teradata"
)

type Backend struct {
	*framework.Backend

	storage     logical.Storage
	mu          sync.RWMutex
	dbRegistry  *storage.DBRegistry
	credCache   *credentialCache
	queryCache  *queryResultCache
	rateLimiter *RateLimiterMiddleware
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

	return nil
}

func (b *Backend) HandleRequest(ctx context.Context, req *logical.Request) (*logical.Response, error) {
	if req == nil {
		return b.Backend.HandleRequest(ctx, req)
	}

	if err := b.rateLimiter.RateLimit(ctx, req); err != nil {
		return nil, err
	}

	return b.Backend.HandleRequest(ctx, req)
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
		b.pathConfigV1(),
		b.pathConfigBackup(),
		b.pathConfigRestore(),
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
		b.pathLeases(),
		b.pathLeaseInfo(),
		b.pathCleanupExpired(),
		b.pathLeaseLookup(),
		b.pathExtendLease(),
		b.pathHealth(),
		b.pathReadiness(),
		b.pathLiveness(),
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

func (b *Backend) Revoke(ctx context.Context, leaseID string) error {
	if leaseID == "" {
		return nil
	}

	parts := strings.Split(leaseID, "/")
	if len(parts) < 4 || parts[0] != "teradata" || parts[1] != "creds" {
		return nil
	}

	roleName := parts[2]
	username := parts[3]

	cred, err := b.getCachedCredential(ctx, b.storage, username)
	if err != nil {
		return fmt.Errorf("failed to get credential for revocation: %w", err)
	}

	var cfg *models.Config
	if cred != nil && cred.Region != "" {
		cfg, err = getConfigByRegion(ctx, b.storage, cred.Region)
		if err != nil {
			return fmt.Errorf("failed to get region config for revocation: %w", err)
		}
		if cfg == nil {
			return fmt.Errorf("configuration for region %q not found", cred.Region)
		}
	} else {
		cfg, err = getConfig(ctx, b.storage)
		if err != nil {
			return fmt.Errorf("failed to get config for revocation: %w", err)
		}
		if cfg == nil {
			return fmt.Errorf("database configuration not found")
		}
	}

	role, err := getRole(ctx, b.storage, roleName)
	if err != nil {
		return fmt.Errorf("failed to get role for revocation: %w", err)
	}

	var revokeSQL string
	if role != nil && role.RevocationStatement != "" {
		revokeSQL = strings.ReplaceAll(role.RevocationStatement, "{{username}}", username)
		var conn *odbc.Connection
		connString := odbc.AppendQueryTimeout(cfg.ConnectionString, cfg.QueryTimeout)
		err = retry.Do(ctx, nil, func() error {
			conn, err = odbc.Connect(connString)
			return err
		})
		if err == nil {
			retry.Do(ctx, nil, func() error {
				return conn.ExecuteMultipleStatements(revokeSQL)
			})
			conn.Close()
		}
	}

	dropSQL := fmt.Sprintf("DROP USER %s", username)
	var conn *odbc.Connection
	connString := odbc.AppendQueryTimeout(cfg.ConnectionString, cfg.QueryTimeout)
	err = retry.Do(ctx, nil, func() error {
		conn, err = odbc.Connect(connString)
		return err
	})
	if err != nil {
		_ = audit.LogCredentialRevocation(ctx, b.storage, username, roleName, map[string]interface{}{"error": err.Error()})
		_ = webhook.SendCredentialRevokedWebhook(ctx, b.storage, username, roleName, map[string]interface{}{"error": err.Error()})
		return fmt.Errorf("failed to connect for revocation: %w", err)
	}
	defer conn.Close()

	err = retry.Do(ctx, nil, func() error {
		return conn.ExecuteMultipleStatements(dropSQL)
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
