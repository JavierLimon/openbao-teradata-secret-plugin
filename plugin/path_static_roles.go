package teradata

import (
	"context"
	"fmt"
	"time"

	"github.com/JavierLimon/openbao-teradata-secret-plugin/models"
	teradb "github.com/JavierLimon/openbao-teradata-secret-plugin/odbc"
	"github.com/openbao/openbao/sdk/v2/framework"
	"github.com/openbao/openbao/sdk/v2/logical"
)

func (b *Backend) pathRotateStaticRole() *framework.Path {
	return &framework.Path{
		Pattern:         "rotate-role/" + framework.GenericNameRegex("name"),
		HelpSynopsis:    "Manually rotate static role password",
		HelpDescription: "Manually rotates the password for a static role.",

		Fields: map[string]*framework.FieldSchema{
			"name": {
				Type:        framework.TypeString,
				Description: "Name of the static role to rotate",
			},
		},

		Operations: map[logical.Operation]framework.OperationHandler{
			logical.UpdateOperation: &framework.PathOperation{
				Callback: b.pathRotateStaticRoleHandler,
			},
		},
	}
}

func (b *Backend) pathStaticRoles() *framework.Path {
	return &framework.Path{
		Pattern:         "static-roles/" + framework.GenericNameRegex("name"),
		HelpSynopsis:    "Manage Teradata static roles",
		HelpDescription: "Create, read, update, and delete static roles that manage database user credentials.",

		Fields: map[string]*framework.FieldSchema{
			"name": {
				Type:        framework.TypeString,
				Description: "Name of the static role",
			},
			"username": {
				Type:        framework.TypeString,
				Description: "Database username for this static role",
				Required:    true,
			},
			"db_name": {
				Type:        framework.TypeString,
				Description: "Database name to use for this static role",
			},
			"rotation_period": {
				Type:        framework.TypeInt,
				Description: "Automatic rotation period in seconds (0 = disabled)",
				Default:     0,
			},
			"rotation_schedule": {
				Type:        framework.TypeString,
				Description: "Cron expression for scheduled rotation",
			},
			"rotation_window": {
				Type:        framework.TypeInt,
				Description: "Rotation window in seconds for scheduled rotation",
				Default:     3600,
			},
			"rotation_statements": {
				Type:        framework.TypeCommaStringSlice,
				Description: "Statements to execute after password rotation",
			},
		},

		Operations: map[logical.Operation]framework.OperationHandler{
			logical.CreateOperation: &framework.PathOperation{
				Callback: b.pathStaticRoleCreate,
			},
			logical.ReadOperation: &framework.PathOperation{
				Callback: b.pathStaticRoleRead,
			},
			logical.UpdateOperation: &framework.PathOperation{
				Callback: b.pathStaticRoleUpdate,
			},
			logical.DeleteOperation: &framework.PathOperation{
				Callback: b.pathStaticRoleDelete,
			},
		},

		ExistenceCheck: b.pathStaticRoleExistenceCheck,
	}
}

func (b *Backend) pathStaticRoleList() *framework.Path {
	return &framework.Path{
		Pattern:         "static-roles",
		HelpSynopsis:    "List Teradata static roles",
		HelpDescription: "Lists all configured Teradata static roles.",

		Operations: map[logical.Operation]framework.OperationHandler{
			logical.ListOperation: &framework.PathOperation{
				Callback: b.pathStaticRoleListHandler,
			},
		},
	}
}

func (b *Backend) pathStaticRoleExistenceCheck(ctx context.Context, req *logical.Request, data *framework.FieldData) (bool, error) {
	name := data.Get("name").(string)

	entry, err := req.Storage.Get(ctx, "static-roles/"+name)
	if err != nil {
		return false, err
	}

	return entry != nil, nil
}

func (b *Backend) pathStaticRoleCreate(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	name := data.Get("name").(string)
	username := data.Get("username").(string)
	dbName := data.Get("db_name").(string)
	rotationPeriod := data.Get("rotation_period").(int)
	rotationSchedule := data.Get("rotation_schedule").(string)
	rotationWindow := data.Get("rotation_window").(int)
	rotationStatements := data.Get("rotation_statements").([]string)

	var cfg *models.Config
	var err error

	if dbName != "" {
		cfg, err = getConfigByName(ctx, req.Storage, dbName)
		if err != nil {
			return nil, err
		}
		if cfg == nil {
			return nil, fmt.Errorf("database configuration %q not found", dbName)
		}
	} else {
		cfg, err = getConfig(ctx, req.Storage)
		if err != nil {
			return nil, err
		}
		if cfg == nil {
			return nil, fmt.Errorf("database configuration not found")
		}
	}

	if err := teradb.ValidateUsername(username); err != nil {
		return nil, fmt.Errorf("invalid username: %w", err)
	}

	staticRole := &models.StaticRole{
		Name:               name,
		Username:           username,
		DBName:             dbName,
		RotationPeriod:     rotationPeriod,
		RotationSchedule:   rotationSchedule,
		RotationWindow:     rotationWindow,
		RotationStatements: rotationStatements,
		LastRotation:       time.Now().Unix(),
		RotationCount:      0,
		Version:            models.StaticRoleVersion,
	}

	entry, err := logical.StorageEntryJSON("static-roles/"+name, staticRole)
	if err != nil {
		return nil, err
	}

	if err := req.Storage.Put(ctx, entry); err != nil {
		return nil, err
	}

	existingCred, _ := getStaticCredential(ctx, req.Storage, name)
	if existingCred == nil {
		password := generatePassword()

		createSQL := fmt.Sprintf("MODIFY USER %s AS PASSWORD = %s", username, password)
		_, err = executeSQL(ctx, cfg, createSQL)
		if err != nil {
			return nil, fmt.Errorf("failed to create initial password for static user: %w", err)
		}

		staticCred := &models.StaticCredential{
			Username:     username,
			Password:     password,
			RoleName:     name,
			DBName:       dbName,
			LastRotated:  time.Now().Unix(),
			NextRotation: computeNextRotation(staticRole),
		}

		credEntry, err := logical.StorageEntryJSON("static-creds/"+name, staticCred)
		if err != nil {
			return nil, err
		}
		if err := req.Storage.Put(ctx, credEntry); err != nil {
			return nil, err
		}
	}

	return &logical.Response{
		Data: map[string]interface{}{
			"name":                name,
			"username":            username,
			"db_name":             dbName,
			"rotation_period":     rotationPeriod,
			"rotation_schedule":   rotationSchedule,
			"rotation_window":     rotationWindow,
			"rotation_statements": rotationStatements,
		},
	}, nil
}

func (b *Backend) pathStaticRoleRead(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	name := data.Get("name").(string)

	entry, err := req.Storage.Get(ctx, "static-roles/"+name)
	if err != nil {
		return nil, err
	}

	if entry == nil {
		return nil, nil
	}

	var staticRole models.StaticRole
	if err := entry.DecodeJSON(&staticRole); err != nil {
		return nil, err
	}

	resp := map[string]interface{}{
		"name":                staticRole.Name,
		"username":            staticRole.Username,
		"db_name":             staticRole.DBName,
		"rotation_period":     staticRole.RotationPeriod,
		"rotation_schedule":   staticRole.RotationSchedule,
		"rotation_window":     staticRole.RotationWindow,
		"rotation_statements": staticRole.RotationStatements,
		"last_rotation":       staticRole.LastRotation,
		"rotation_count":      staticRole.RotationCount,
		"version":             staticRole.Version,
	}

	return &logical.Response{
		Data: resp,
	}, nil
}

func (b *Backend) pathStaticRoleUpdate(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	name := data.Get("name").(string)

	existingRole, err := getStaticRole(ctx, req.Storage, name)
	if err != nil {
		return nil, err
	}

	if existingRole == nil {
		return nil, fmt.Errorf("static role %q not found", name)
	}

	username := data.Get("username").(string)
	if username != "" {
		if err := teradb.ValidateUsername(username); err != nil {
			return nil, fmt.Errorf("invalid username: %w", err)
		}
		existingRole.Username = username
	}

	if dbName, ok := data.Raw["db_name"].(string); ok {
		existingRole.DBName = dbName
	}

	if rotationPeriod, ok := data.Raw["rotation_period"].(int); ok {
		existingRole.RotationPeriod = rotationPeriod
	}

	if rotationSchedule, ok := data.Raw["rotation_schedule"].(string); ok {
		existingRole.RotationSchedule = rotationSchedule
	}

	if rotationWindow, ok := data.Raw["rotation_window"].(int); ok {
		existingRole.RotationWindow = rotationWindow
	}

	if rotationStatements, ok := data.Raw["rotation_statements"].([]string); ok {
		existingRole.RotationStatements = rotationStatements
	}

	entry, err := logical.StorageEntryJSON("static-roles/"+name, existingRole)
	if err != nil {
		return nil, err
	}

	if err := req.Storage.Put(ctx, entry); err != nil {
		return nil, err
	}

	return &logical.Response{
		Data: map[string]interface{}{
			"name":                existingRole.Name,
			"username":            existingRole.Username,
			"db_name":             existingRole.DBName,
			"rotation_period":     existingRole.RotationPeriod,
			"rotation_schedule":   existingRole.RotationSchedule,
			"rotation_window":     existingRole.RotationWindow,
			"rotation_statements": existingRole.RotationStatements,
		},
	}, nil
}

func (b *Backend) pathStaticRoleDelete(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	name := data.Get("name").(string)

	staticRole, err := getStaticRole(ctx, req.Storage, name)
	if err != nil {
		return nil, err
	}

	if staticRole == nil {
		return nil, nil
	}

	cfg, err := getConfig(ctx, req.Storage)
	if err != nil {
		return nil, err
	}

	if cfg != nil {
		dropSQL := fmt.Sprintf("DROP USER %s", staticRole.Username)
		executeSQL(ctx, cfg, dropSQL)
	}

	err = req.Storage.Delete(ctx, "static-roles/"+name)
	if err != nil {
		return nil, fmt.Errorf("error deleting static role: %w", err)
	}

	req.Storage.Delete(ctx, "static-creds/"+name)

	return nil, nil
}

func (b *Backend) pathStaticRoleListHandler(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	entries, err := req.Storage.List(ctx, "static-roles/")
	if err != nil {
		return nil, err
	}

	return logical.ListResponse(entries), nil
}

func (b *Backend) pathRotateStaticRoleHandler(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	name := data.Get("name").(string)

	staticRole, err := getStaticRole(ctx, req.Storage, name)
	if err != nil {
		return nil, err
	}

	if staticRole == nil {
		return nil, fmt.Errorf("static role %q not found", name)
	}

	cfg, err := getConfig(ctx, req.Storage)
	if err != nil {
		return nil, err
	}
	if cfg == nil {
		return nil, fmt.Errorf("database configuration not found")
	}

	password := generatePassword()

	updateSQL := fmt.Sprintf("MODIFY USER %s AS PASSWORD = %s", staticRole.Username, password)
	_, err = executeSQL(ctx, cfg, updateSQL)
	if err != nil {
		return nil, fmt.Errorf("failed to rotate password: %w", err)
	}

	if len(staticRole.RotationStatements) > 0 {
		for _, stmt := range staticRole.RotationStatements {
			stmt = replaceStaticRoleVars(stmt, staticRole.Username, password)
			executeSQL(ctx, cfg, stmt)
		}
	}

	staticRole.LastRotation = time.Now().Unix()
	staticRole.RotationCount++

	entry, err := logical.StorageEntryJSON("static-roles/"+name, staticRole)
	if err != nil {
		return nil, err
	}
	if err := req.Storage.Put(ctx, entry); err != nil {
		return nil, err
	}

	staticCred := &models.StaticCredential{
		Username:     staticRole.Username,
		Password:     password,
		RoleName:     name,
		DBName:       staticRole.DBName,
		LastRotated:  time.Now().Unix(),
		NextRotation: computeNextRotation(staticRole),
	}

	credEntry, err := logical.StorageEntryJSON("static-creds/"+name, staticCred)
	if err != nil {
		return nil, err
	}
	if err := req.Storage.Put(ctx, credEntry); err != nil {
		return nil, err
	}

	return &logical.Response{
		Data: map[string]interface{}{
			"username":       staticRole.Username,
			"password":       password,
			"last_rotated":   staticRole.LastRotation,
			"rotation_count": staticRole.RotationCount,
		},
	}, nil
}

func getStaticRole(ctx context.Context, storage logical.Storage, name string) (*models.StaticRole, error) {
	entry, err := storage.Get(ctx, "static-roles/"+name)
	if err != nil {
		return nil, err
	}

	if entry == nil {
		return nil, nil
	}

	var staticRole models.StaticRole
	if err := entry.DecodeJSON(&staticRole); err != nil {
		return nil, err
	}

	return &staticRole, nil
}

func getStaticCredential(ctx context.Context, storage logical.Storage, name string) (*models.StaticCredential, error) {
	entry, err := storage.Get(ctx, "static-creds/"+name)
	if err != nil {
		return nil, err
	}

	if entry == nil {
		return nil, nil
	}

	var staticCred models.StaticCredential
	if err := entry.DecodeJSON(&staticCred); err != nil {
		return nil, err
	}

	return &staticCred, nil
}

func computeNextRotation(staticRole *models.StaticRole) int64 {
	if staticRole.RotationPeriod > 0 {
		return staticRole.LastRotation + int64(staticRole.RotationPeriod)
	}
	return 0
}

func replaceStaticRoleVars(stmt, username, password string) string {
	stmt = fmt.Sprintf("REPLACE ALL OCCURRENCES OF '{{username}}' IN STR(%s) WITH '%s'.", stmt, username)
	stmt = fmt.Sprintf("REPLACE ALL OCCURRENCES OF '{{password}}' IN STR(%s) WITH '%s'.", stmt, password)
	return stmt
}
