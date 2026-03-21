package teradata

import (
	"context"
	"sync"

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
		b.pathRoles(),
		b.pathRoleList(),
		b.pathStatements(),
		b.pathStatementList(),
		b.pathRotateRoot(),
		b.pathCreds(),
		b.pathCredsBatch(),
		b.pathHealth(),
		b.pathVersion(),
		b.pathPoolStats(),
	}
}

func (b *Backend) getDBRegistry() *storage.DBRegistry {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.dbRegistry
}

const backendHelp = `
The Teradata secrets engine provides dynamic database credentials
for Teradata databases using ODBC connectivity.

Once configured, roles can be created that define which database
user will be created and what permissions they should have.
`
