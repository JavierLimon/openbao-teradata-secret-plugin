package teradata

import (
	"context"
	"fmt"

	"github.com/JavierLimon/openbao-teradata-secret-plugin/audit"
	"github.com/JavierLimon/openbao-teradata-secret-plugin/models"
	"github.com/JavierLimon/openbao-teradata-secret-plugin/security"
	"github.com/JavierLimon/openbao-teradata-secret-plugin/webhook"
	"github.com/openbao/openbao/sdk/v2/framework"
	"github.com/openbao/openbao/sdk/v2/logical"
)

func (b *Backend) pathRoles() *framework.Path {
	return &framework.Path{
		Pattern:         "roles/" + framework.GenericNameRegex("name"),
		HelpSynopsis:    "Manage Teradata roles",
		HelpDescription: "Create, read, update, and delete roles that define database user credentials.",

		Fields: map[string]*framework.FieldSchema{
			"name": {
				Type:        framework.TypeString,
				Description: "Name of the role",
			},
			"version": {
				Type:        framework.TypeInt,
				Description: "Role schema version",
			},
			"db_user": {
				Type:        framework.TypeString,
				Description: "Database username template (use {{username}} placeholder)",
				Required:    true,
			},
			"db_password": {
				Type:        framework.TypeString,
				Description: "Database password template (use {{password}} placeholder)",
			},
			"default_ttl": {
				Type:        framework.TypeInt,
				Description: "Default lease duration in seconds",
				Default:     3600,
			},
			"max_ttl": {
				Type:        framework.TypeInt,
				Description: "Maximum lease duration in seconds",
				Default:     86400,
			},
			"renewal_period": {
				Type:        framework.TypeInt,
				Description: "Automatic renewal period in seconds (0 = disabled)",
				Default:     0,
			},
			"statement_template": {
				Type:        framework.TypeString,
				Description: "Name of the statement template to use",
			},
			"default_database": {
				Type:        framework.TypeString,
				Description: "Default database for the user",
				Default:     "USER",
			},
			"perm_space": {
				Type:        framework.TypeInt,
				Description: "Permanent space in bytes (0 = unlimited from owner)",
				Default:     0,
			},
			"spool_space": {
				Type:        framework.TypeInt,
				Description: "Spool space in bytes (0 = default)",
				Default:     0,
			},
			"account": {
				Type:        framework.TypeString,
				Description: "Account string",
			},
			"fallback": {
				Type:        framework.TypeBool,
				Description: "Enable fallback protection",
				Default:     false,
			},
			"creation_statement": {
				Type:        framework.TypeString,
				Description: "Additional SQL to run after CREATE USER",
			},
			"revocation_statement": {
				Type:        framework.TypeString,
				Description: "Additional SQL to run before DROP USER",
				Default:     "",
			},
			"rollback_statement": {
				Type:        framework.TypeString,
				Description: "SQL to run if creation statement fails",
				Default:     "",
			},
			"renewal_statement": {
				Type:        framework.TypeString,
				Description: "SQL to run after password renewal",
				Default:     "",
			},
			"max_credentials": {
				Type:        framework.TypeInt,
				Description: "Maximum number of credentials allowed for this role (0 = unlimited)",
				Default:     0,
			},
			"session_variables": {
				Type:        framework.TypeMap,
				Description: "Session variables to set for user sessions (map of key-value pairs)",
			},
		},

		Operations: map[logical.Operation]framework.OperationHandler{
			logical.CreateOperation: &framework.PathOperation{
				Callback: b.pathRoleCreate,
			},
			logical.ReadOperation: &framework.PathOperation{
				Callback: b.pathRoleRead,
			},
			logical.UpdateOperation: &framework.PathOperation{
				Callback: b.pathRoleUpdate,
			},
			logical.DeleteOperation: &framework.PathOperation{
				Callback: b.pathRoleDelete,
			},
		},

		ExistenceCheck: b.pathRoleExistenceCheck,
	}
}

func (b *Backend) pathRoleList() *framework.Path {
	return &framework.Path{
		Pattern:         "roles",
		HelpSynopsis:    "List Teradata roles",
		HelpDescription: "Lists all configured Teradata roles.",

		Operations: map[logical.Operation]framework.OperationHandler{
			logical.ListOperation: &framework.PathOperation{
				Callback: b.pathRoleListHandler,
			},
		},
	}
}

func (b *Backend) pathRoleExistenceCheck(ctx context.Context, req *logical.Request, data *framework.FieldData) (bool, error) {
	name := data.Get("name").(string)

	entry, err := req.Storage.Get(ctx, "roles/"+name)
	if err != nil {
		return false, err
	}

	return entry != nil, nil
}

func (b *Backend) pathRoleCreate(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	name := data.Get("name").(string)

	dbUser := data.Get("db_user").(string)

	creationStatement := data.Get("creation_statement").(string)
	revocationStatement := data.Get("revocation_statement").(string)
	rollbackStatement := data.Get("rollback_statement").(string)
	renewalStatement := data.Get("renewal_statement").(string)

	if err := security.ValidateStatementTemplates(creationStatement, revocationStatement, rollbackStatement, renewalStatement); err != nil {
		return nil, fmt.Errorf("SQL statement validation failed: %w", err)
	}

	var sessionVariables map[string]string
	if rawVars, ok := data.Raw["session_variables"].(map[string]interface{}); ok {
		sessionVariables = make(map[string]string)
		for k, v := range rawVars {
			if strVal, ok := v.(string); ok {
				sessionVariables[k] = strVal
			}
		}
	}

	if err := security.ValidateSessionVariables(sessionVariables); err != nil {
		return nil, fmt.Errorf("invalid session_variables: %w", err)
	}

	role := &models.Role{
		Name:                name,
		Version:             models.RoleVersion,
		DBUser:              dbUser,
		DBPassword:          data.Get("db_password").(string),
		DefaultTTL:          data.Get("default_ttl").(int),
		MaxTTL:              data.Get("max_ttl").(int),
		RenewalPeriod:       data.Get("renewal_period").(int),
		StatementTemplate:   data.Get("statement_template").(string),
		CreationStatement:   creationStatement,
		RevocationStatement: revocationStatement,
		RollbackStatement:   rollbackStatement,
		RenewalStatement:    renewalStatement,
		MaxCredentials:      data.Get("max_credentials").(int),
		SessionVariables:    sessionVariables,
	}

	entry, err := logical.StorageEntryJSON("roles/"+name, role)
	if err != nil {
		return nil, err
	}

	if err := req.Storage.Put(ctx, entry); err != nil {
		return nil, err
	}

	b.invalidateRoleCache(name)

	_ = audit.LogRoleCreation(ctx, req.Storage, name, role.DBUser, role.StatementTemplate)
	_ = webhook.SendRoleCreatedWebhook(ctx, req.Storage, name, role.DBUser, role.StatementTemplate)

	return &logical.Response{
		Data: map[string]interface{}{
			"name":                 name,
			"db_user":              role.DBUser,
			"default_ttl":          role.DefaultTTL,
			"max_ttl":              role.MaxTTL,
			"renewal_period":       role.RenewalPeriod,
			"statement_template":   role.StatementTemplate,
			"creation_statement":   "***",
			"revocation_statement": "***",
			"max_credentials":      role.MaxCredentials,
		},
	}, nil
}

func (b *Backend) pathRoleRead(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	name := data.Get("name").(string)

	role, err := getRole(ctx, req.Storage, name)
	if err != nil {
		return nil, err
	}

	if role == nil {
		return nil, nil
	}

	migrated, err := migrateRole(ctx, req.Storage, role)
	if err != nil {
		return nil, err
	}

	if migrated.Version != role.Version {
		entry, err := logical.StorageEntryJSON("roles/"+name, migrated)
		if err != nil {
			return nil, err
		}
		if err := req.Storage.Put(ctx, entry); err != nil {
			return nil, err
		}
		role = migrated
	}

	resp := map[string]interface{}{
		"name":                 role.Name,
		"version":              role.Version,
		"db_user":              role.DBUser,
		"default_ttl":          role.DefaultTTL,
		"max_ttl":              role.MaxTTL,
		"renewal_period":       role.RenewalPeriod,
		"statement_template":   role.StatementTemplate,
		"creation_statement":   role.CreationStatement,
		"revocation_statement": role.RevocationStatement,
		"rollback_statement":   role.RollbackStatement,
		"renewal_statement":    role.RenewalStatement,
		"max_credentials":      role.MaxCredentials,
		"session_variables":    role.SessionVariables,
	}

	if role.DBPassword != "" {
		resp["db_password"] = "***"
	}

	return &logical.Response{
		Data: resp,
	}, nil
}

func (b *Backend) pathRoleUpdate(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	name := data.Get("name").(string)

	existingRole, err := getRole(ctx, req.Storage, name)
	if err != nil {
		return nil, err
	}

	dbUser := data.Get("db_user").(string)

	creationStatement := data.Get("creation_statement").(string)
	revocationStatement := data.Get("revocation_statement").(string)
	rollbackStatement := data.Get("rollback_statement").(string)
	renewalStatement := data.Get("renewal_statement").(string)

	if err := security.ValidateStatementTemplates(creationStatement, revocationStatement, rollbackStatement, renewalStatement); err != nil {
		return nil, fmt.Errorf("SQL statement validation failed: %w", err)
	}

	var sessionVariables map[string]string
	if rawVars, ok := data.Raw["session_variables"].(map[string]interface{}); ok {
		sessionVariables = make(map[string]string)
		for k, v := range rawVars {
			if strVal, ok := v.(string); ok {
				sessionVariables[k] = strVal
			}
		}
	}

	if err := security.ValidateSessionVariables(sessionVariables); err != nil {
		return nil, fmt.Errorf("invalid session_variables: %w", err)
	}

	role := &models.Role{
		Name:                name,
		Version:             models.RoleVersion,
		DBUser:              dbUser,
		DBPassword:          data.Get("db_password").(string),
		DefaultTTL:          data.Get("default_ttl").(int),
		MaxTTL:              data.Get("max_ttl").(int),
		RenewalPeriod:       data.Get("renewal_period").(int),
		StatementTemplate:   data.Get("statement_template").(string),
		CreationStatement:   creationStatement,
		RevocationStatement: revocationStatement,
		RollbackStatement:   rollbackStatement,
		RenewalStatement:    renewalStatement,
		MaxCredentials:      data.Get("max_credentials").(int),
		SessionVariables:    sessionVariables,
	}

	if existingRole != nil {
		role.UsernamePrefix = existingRole.UsernamePrefix
		role.DefaultDatabase = existingRole.DefaultDatabase
		role.PermSpace = existingRole.PermSpace
		role.SpoolSpace = existingRole.SpoolSpace
		role.Account = existingRole.Account
		role.Fallback = existingRole.Fallback
		role.BatchSize = existingRole.BatchSize
		if role.SessionVariables == nil {
			role.SessionVariables = existingRole.SessionVariables
		}
	}

	entry, err := logical.StorageEntryJSON("roles/"+name, role)
	if err != nil {
		return nil, err
	}

	if err := req.Storage.Put(ctx, entry); err != nil {
		return nil, err
	}

	b.invalidateRoleCache(name)

	_ = audit.LogRoleUpdate(ctx, req.Storage, name, role.DBUser, role.StatementTemplate)
	_ = webhook.SendRoleUpdatedWebhook(ctx, req.Storage, name, role.DBUser, role.StatementTemplate)

	return &logical.Response{
		Data: map[string]interface{}{
			"name":            name,
			"db_user":         role.DBUser,
			"default_ttl":     role.DefaultTTL,
			"max_ttl":         role.MaxTTL,
			"renewal_period":  role.RenewalPeriod,
			"max_credentials": role.MaxCredentials,
		},
	}, nil
}

func (b *Backend) pathRoleDelete(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	name := data.Get("name").(string)

	err := req.Storage.Delete(ctx, "roles/"+name)
	if err != nil {
		return nil, fmt.Errorf("error deleting role: %w", err)
	}

	b.invalidateRoleCache(name)

	_ = audit.LogRoleDeletion(ctx, req.Storage, name)
	_ = webhook.SendRoleDeletedWebhook(ctx, req.Storage, name)

	return nil, nil
}

func (b *Backend) pathRoleListHandler(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	entries, err := req.Storage.List(ctx, "roles/")
	if err != nil {
		return nil, err
	}

	return logical.ListResponse(entries), nil
}

func getRole(ctx context.Context, storage logical.Storage, name string) (*models.Role, error) {
	entry, err := storage.Get(ctx, "roles/"+name)
	if err != nil {
		return nil, err
	}

	if entry == nil {
		return nil, nil
	}

	var role models.Role
	if err := entry.DecodeJSON(&role); err != nil {
		return nil, err
	}

	migratedRole, err := migrateRole(ctx, storage, &role)
	if err != nil {
		return nil, err
	}

	return migratedRole, nil
}
