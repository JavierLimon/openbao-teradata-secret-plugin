package teradata

import (
	"github.com/openbao/openbao/sdk/v2/framework"
	"github.com/openbao/openbao/sdk/v2/logical"
)

func (b *Backend) pathConfigV1() *framework.Path {
	return &framework.Path{
		Pattern:         "v1/config",
		HelpSynopsis:    "Configure the Teradata connection (v1)",
		HelpDescription: "Configures the connection parameters for the Teradata database. This is the v1 API version.",

		Fields: map[string]*framework.FieldSchema{
			"connection_string": {
				Type:        framework.TypeString,
				Description: "ODBC connection string for Teradata",
				Required:    true,
			},
			"max_open_connections": {
				Type:        framework.TypeInt,
				Description: "Maximum number of open connections",
				Default:     5,
			},
			"max_idle_connections": {
				Type:        framework.TypeInt,
				Description: "Maximum number of idle connections",
				Default:     2,
			},
			"connection_timeout": {
				Type:        framework.TypeInt,
				Description: "Connection timeout in seconds",
				Default:     30,
			},
		},

		Operations: map[logical.Operation]framework.OperationHandler{
			logical.CreateOperation: &framework.PathOperation{
				Callback: b.pathConfigWrite,
			},
			logical.ReadOperation: &framework.PathOperation{
				Callback: b.pathConfigRead,
			},
			logical.UpdateOperation: &framework.PathOperation{
				Callback: b.pathConfigWrite,
			},
			logical.DeleteOperation: &framework.PathOperation{
				Callback: b.pathConfigDelete,
			},
		},
	}
}

func (b *Backend) pathRolesV1() *framework.Path {
	return &framework.Path{
		Pattern:         "v1/roles/" + framework.GenericNameRegex("name"),
		HelpSynopsis:    "Manage Teradata roles (v1)",
		HelpDescription: "Create, read, update, and delete roles that define database user credentials. This is the v1 API version.",

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

func (b *Backend) pathRoleListV1() *framework.Path {
	return &framework.Path{
		Pattern:         "v1/roles",
		HelpSynopsis:    "List Teradata roles (v1)",
		HelpDescription: "Lists all configured Teradata roles. This is the v1 API version.",

		Operations: map[logical.Operation]framework.OperationHandler{
			logical.ListOperation: &framework.PathOperation{
				Callback: b.pathRoleListHandler,
			},
		},
	}
}

func (b *Backend) pathRotateRootV1() *framework.Path {
	return &framework.Path{
		Pattern:         "v1/rotate-root",
		HelpSynopsis:    "Rotate root credentials (v1)",
		HelpDescription: "Rotates the root database credentials used for administrative tasks. This is the v1 API version.",

		Operations: map[logical.Operation]framework.OperationHandler{
			logical.UpdateOperation: &framework.PathOperation{
				Callback: b.pathRotateRootHandler,
			},
		},
	}
}

func (b *Backend) pathCredsV1() *framework.Path {
	return &framework.Path{
		Pattern:         "v1/creds/" + framework.GenericNameRegex("name"),
		HelpSynopsis:    "Generate database credentials (v1)",
		HelpDescription: "Generates dynamic database credentials for the specified role. This is the v1 API version.",

		Fields: map[string]*framework.FieldSchema{
			"name": {
				Type:        framework.TypeString,
				Description: "Name of the role",
			},
		},

		Operations: map[logical.Operation]framework.OperationHandler{
			logical.ReadOperation: &framework.PathOperation{
				Callback: b.pathCredsRead,
			},
		},
	}
}
