package teradata

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/JavierLimon/openbao-teradata-secret-plugin/audit"
	teradb "github.com/JavierLimon/openbao-teradata-secret-plugin/odbc"
	"github.com/JavierLimon/openbao-teradata-secret-plugin/storage"
	"github.com/openbao/openbao/sdk/v2/framework"
	"github.com/openbao/openbao/sdk/v2/logical"
)

const (
	OperationPrefix = "teradata"
)

type Backend struct {
	*framework.Backend

	storage logical.Storage
	mu      sync.RWMutex

	dbRegistry *storage.DBRegistry
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

	return nil
}

func (b *Backend) paths() []*framework.Path {
	return []*framework.Path{
		b.pathConfig(),
		b.pathConfigV1(),
		b.pathConfigBackup(),
		b.pathConfigRestore(),
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
		b.pathRevokeCreds(),
		b.pathLeases(),
		b.pathLeaseInfo(),
		b.pathCleanupExpired(),
		b.pathLeaseLookup(),
		b.pathHealth(),
		b.pathVersion(),
		b.pathAPIVersion(),
		b.pathPoolStats(),
	}
}

func (b *Backend) getDBRegistry() *storage.DBRegistry {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.dbRegistry
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

	cfg, err := getConfig(ctx, b.storage)
	if err != nil {
		return fmt.Errorf("failed to get config for revocation: %w", err)
	}
	if cfg == nil {
		return fmt.Errorf("database configuration not found")
	}

	role, err := getRole(ctx, b.storage, roleName)
	if err != nil {
		return fmt.Errorf("failed to get role for revocation: %w", err)
	}

	var revokeSQL string
	if role != nil && role.RevocationStatement != "" {
		revokeSQL = strings.ReplaceAll(role.RevocationStatement, "{{username}}", username)
		conn, err := teradb.Connect(cfg.ConnectionString)
		if err == nil {
			conn.ExecuteMultipleStatements(revokeSQL)
			conn.Close()
		}
	}

	dropSQL := fmt.Sprintf("DROP USER %s", username)
	conn, err := teradb.Connect(cfg.ConnectionString)
	if err != nil {
		_ = audit.LogCredentialRevocation(ctx, b.storage, username, roleName, map[string]interface{}{"error": err.Error()})
		return fmt.Errorf("failed to connect for revocation: %w", err)
	}
	defer conn.Close()

	err = conn.ExecuteMultipleStatements(dropSQL)
	if err != nil {
		_ = audit.LogCredentialRevocation(ctx, b.storage, username, roleName, map[string]interface{}{"error": err.Error()})
		return fmt.Errorf("failed to revoke credential: %w", err)
	}

	_ = audit.LogCredentialRevocation(ctx, b.storage, username, roleName, nil)
	return nil
}

const backendHelp = `
The Teradata secrets engine provides dynamic database credentials
for Teradata databases using ODBC connectivity.

Once configured, roles can be created that define which database
user will be created and what permissions they should have.
`
