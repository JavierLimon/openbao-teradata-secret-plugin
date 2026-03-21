package teradata

import (
	"context"
	"fmt"

	"github.com/JavierLimon/openbao-teradata-secret-plugin/audit"
	"github.com/JavierLimon/openbao-teradata-secret-plugin/models"
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

	role := &models.Role{
		Name:                name,
		DBUser:              data.Get("db_user").(string),
		DBPassword:          data.Get("db_password").(string),
		DefaultTTL:          data.Get("default_ttl").(int),
		MaxTTL:              data.Get("max_ttl").(int),
		StatementTemplate:   data.Get("statement_template").(string),
		CreationStatement:   data.Get("creation_statement").(string),
		RevocationStatement: data.Get("revocation_statement").(string),
		RollbackStatement:   data.Get("rollback_statement").(string),
		RenewalStatement:    data.Get("renewal_statement").(string),
	}

	entry, err := logical.StorageEntryJSON("roles/"+name, role)
	if err != nil {
		return nil, err
	}

	if err := req.Storage.Put(ctx, entry); err != nil {
		return nil, err
	}

	_ = audit.LogRoleCreation(ctx, req.Storage, name, role.DBUser, role.StatementTemplate)

	return &logical.Response{
		Data: map[string]interface{}{
			"name":                 name,
			"db_user":              role.DBUser,
			"default_ttl":          role.DefaultTTL,
			"max_ttl":              role.MaxTTL,
			"statement_template":   role.StatementTemplate,
			"creation_statement":   "***",
			"revocation_statement": "***",
		},
	}, nil
}

func (b *Backend) pathRoleRead(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	name := data.Get("name").(string)

	entry, err := req.Storage.Get(ctx, "roles/"+name)
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

	resp := map[string]interface{}{
		"name":                 role.Name,
		"db_user":              role.DBUser,
		"default_ttl":          role.DefaultTTL,
		"max_ttl":              role.MaxTTL,
		"statement_template":   role.StatementTemplate,
		"creation_statement":   role.CreationStatement,
		"revocation_statement": role.RevocationStatement,
		"rollback_statement":   role.RollbackStatement,
		"renewal_statement":    role.RenewalStatement,
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

	role := &models.Role{
		Name:                name,
		DBUser:              data.Get("db_user").(string),
		DBPassword:          data.Get("db_password").(string),
		DefaultTTL:          data.Get("default_ttl").(int),
		MaxTTL:              data.Get("max_ttl").(int),
		StatementTemplate:   data.Get("statement_template").(string),
		CreationStatement:   data.Get("creation_statement").(string),
		RevocationStatement: data.Get("revocation_statement").(string),
		RollbackStatement:   data.Get("rollback_statement").(string),
		RenewalStatement:    data.Get("renewal_statement").(string),
	}

	entry, err := logical.StorageEntryJSON("roles/"+name, role)
	if err != nil {
		return nil, err
	}

	if err := req.Storage.Put(ctx, entry); err != nil {
		return nil, err
	}

	_ = audit.LogRoleUpdate(ctx, req.Storage, name, role.DBUser, role.StatementTemplate)

	return &logical.Response{
		Data: map[string]interface{}{
			"name":        name,
			"db_user":     role.DBUser,
			"default_ttl": role.DefaultTTL,
			"max_ttl":     role.MaxTTL,
		},
	}, nil
}

func (b *Backend) pathRoleDelete(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	name := data.Get("name").(string)

	err := req.Storage.Delete(ctx, "roles/"+name)
	if err != nil {
		return nil, fmt.Errorf("error deleting role: %w", err)
	}

	_ = audit.LogRoleDeletion(ctx, req.Storage, name)

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

	return &role, nil
}
