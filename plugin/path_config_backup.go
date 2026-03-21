package teradata

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/JavierLimon/openbao-teradata-secret-plugin/models"
	"github.com/openbao/openbao/sdk/v2/framework"
	"github.com/openbao/openbao/sdk/v2/logical"
)

func (b *Backend) pathConfigBackup() *framework.Path {
	return &framework.Path{
		Pattern:         "config/backup",
		HelpSynopsis:    "Backup configuration",
		HelpDescription: "Creates a backup of the Teradata plugin configuration.",

		Fields: map[string]*framework.FieldSchema{},

		Operations: map[logical.Operation]framework.OperationHandler{
			logical.ReadOperation: &framework.PathOperation{
				Callback: b.pathConfigBackupRead,
			},
		},
	}
}

func (b *Backend) pathConfigRestore() *framework.Path {
	return &framework.Path{
		Pattern:         "config/restore",
		HelpSynopsis:    "Restore configuration",
		HelpDescription: "Restores the Teradata plugin configuration from a backup.",

		Fields: map[string]*framework.FieldSchema{
			"backup": {
				Type:        framework.TypeString,
				Description: "Base64 encoded backup data",
				Required:    true,
			},
		},

		Operations: map[logical.Operation]framework.OperationHandler{
			logical.CreateOperation: &framework.PathOperation{
				Callback: b.pathConfigRestoreWrite,
			},
			logical.UpdateOperation: &framework.PathOperation{
				Callback: b.pathConfigRestoreWrite,
			},
		},
	}
}

type ConfigBackup struct {
	Config     *models.Config               `json:"config"`
	Roles      map[string]*models.Role      `json:"roles"`
	Statements map[string]*models.Statement `json:"statements"`
	Version    string                       `json:"version"`
	PluginName string                       `json:"plugin_name"`
}

func (b *Backend) pathConfigBackupRead(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	cfg, err := getConfig(ctx, b.storage)
	if err != nil {
		return nil, fmt.Errorf("failed to get config: %w", err)
	}

	roleNames, err := b.storage.List(ctx, "roles/")
	if err != nil {
		return nil, fmt.Errorf("failed to list roles: %w", err)
	}

	roleData := make(map[string]*models.Role)
	for _, roleName := range roleNames {
		role, err := getRole(ctx, b.storage, roleName)
		if err != nil {
			return nil, fmt.Errorf("failed to get role %s: %w", roleName, err)
		}
		if role != nil {
			roleData[roleName] = role
		}
	}

	statementNames, err := b.storage.List(ctx, "statements/")
	if err != nil {
		return nil, fmt.Errorf("failed to list statements: %w", err)
	}

	statementData := make(map[string]*models.Statement)
	for _, stmtName := range statementNames {
		stmt, err := getStatement(ctx, b.storage, stmtName)
		if err != nil {
			return nil, fmt.Errorf("failed to get statement %s: %w", stmtName, err)
		}
		if stmt != nil {
			statementData[stmtName] = stmt
		}
	}

	backup := ConfigBackup{
		Config:     cfg,
		Roles:      roleData,
		Statements: statementData,
		Version:    "1.0.0",
		PluginName: "teradata",
	}

	backupJSON, err := json.Marshal(backup)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal backup: %w", err)
	}

	backupEncoded := bytes.NewBuffer(nil)
	encoder := json.NewEncoder(backupEncoded)
	encoder.SetEscapeHTML(false)
	err = encoder.Encode(backup)
	if err != nil {
		return nil, fmt.Errorf("failed to encode backup: %w", err)
	}

	return &logical.Response{
		Data: map[string]interface{}{
			"backup":           backupJSON,
			"version":          backup.Version,
			"plugin_name":      backup.PluginName,
			"roles_count":      len(roleData),
			"statements_count": len(statementData),
			"config_exists":    cfg != nil,
		},
	}, nil
}

func (b *Backend) pathConfigRestoreWrite(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	backupStr := data.Get("backup").(string)
	if backupStr == "" {
		return logical.ErrorResponse("backup data is required"), nil
	}

	var backup ConfigBackup
	err := json.Unmarshal([]byte(backupStr), &backup)
	if err != nil {
		return logical.ErrorResponse("invalid backup format: %w", err), nil
	}

	if backup.PluginName != "teradata" {
		return logical.ErrorResponse("backup is not from teradata plugin"), nil
	}

	if backup.Config != nil {
		entry, err := logical.StorageEntryJSON("config", backup.Config)
		if err != nil {
			return nil, fmt.Errorf("failed to create storage entry: %w", err)
		}

		if err := b.storage.Put(ctx, entry); err != nil {
			return nil, fmt.Errorf("failed to restore config: %w", err)
		}
	}

	for roleName, role := range backup.Roles {
		entry, err := logical.StorageEntryJSON("roles/"+roleName, role)
		if err != nil {
			return nil, fmt.Errorf("failed to create storage entry for role %s: %w", roleName, err)
		}

		if err := b.storage.Put(ctx, entry); err != nil {
			return nil, fmt.Errorf("failed to restore role %s: %w", roleName, err)
		}
	}

	for stmtName, stmt := range backup.Statements {
		entry, err := logical.StorageEntryJSON("statements/"+stmtName, stmt)
		if err != nil {
			return nil, fmt.Errorf("failed to create storage entry for statement %s: %w", stmtName, err)
		}

		if err := b.storage.Put(ctx, entry); err != nil {
			return nil, fmt.Errorf("failed to restore statement %s: %w", stmtName, err)
		}
	}

	return &logical.Response{
		Data: map[string]interface{}{
			"restored":            true,
			"config_restored":     backup.Config != nil,
			"roles_restored":      len(backup.Roles),
			"statements_restored": len(backup.Statements),
			"version":             backup.Version,
		},
	}, nil
}
