package teradata

import (
	"context"
	"fmt"

	"github.com/JavierLimon/openbao-teradata-secret-plugin/models"
	"github.com/openbao/openbao/sdk/v2/logical"
)

type roleMigrator func(ctx context.Context, storage logical.Storage, role *models.Role) (*models.Role, error)

var roleMigrations = map[int]roleMigrator{
	1: migrateRoleV1toV2,
	2: migrateRoleV2toV3,
}

func migrateRoleV1toV2(ctx context.Context, storage logical.Storage, role *models.Role) (*models.Role, error) {
	if role.DefaultDatabase == "" {
		role.DefaultDatabase = "USER"
	}
	if role.PermSpace == 0 {
		role.PermSpace = 10485760
	}
	if role.SpoolSpace == 0 {
		role.SpoolSpace = 10485760
	}
	role.Version = 2
	return role, nil
}

func migrateRoleV2toV3(ctx context.Context, storage logical.Storage, role *models.Role) (*models.Role, error) {
	role.Version = 3
	return role, nil
}

func migrateRole(ctx context.Context, storage logical.Storage, role *models.Role) (*models.Role, error) {
	if role.Version == 0 {
		role.Version = 1
	}

	if role.Version < models.RoleVersion {
		for v := role.Version; v < models.RoleVersion; v++ {
			migrator, exists := roleMigrations[v]
			if !exists {
				return nil, fmt.Errorf("no migration path from version %d to %d", v, models.RoleVersion)
			}
			var err error
			role, err = migrator(ctx, storage, role)
			if err != nil {
				return nil, fmt.Errorf("migration failed from v%d to v%d: %w", v, v+1, err)
			}
		}
	}

	return role, nil
}
