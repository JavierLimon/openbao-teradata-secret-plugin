package teradata

import (
	"context"
	"fmt"
	"time"

	"github.com/JavierLimon/openbao-teradata-secret-plugin/models"
	teradb "github.com/JavierLimon/openbao-teradata-secret-plugin/odbc"
	"github.com/JavierLimon/openbao-teradata-secret-plugin/security"
	"github.com/JavierLimon/openbao-teradata-secret-plugin/storage"
	"github.com/openbao/openbao/sdk/v2/framework"
	"github.com/openbao/openbao/sdk/v2/logical"
)

func (b *Backend) pathConfig() *framework.Path {
	return &framework.Path{
		Pattern:         "config/" + framework.GenericNameRegex("name"),
		HelpSynopsis:    "Configure the Teradata connection for a specific name",
		HelpDescription: "Configures the connection parameters for a specific Teradata database connection.",

		Fields: map[string]*framework.FieldSchema{
			"name": {
				Type:        framework.TypeString,
				Description: "Name identifier for the connection",
			},
			"plugin_name": {
				Type:        framework.TypeString,
				Description: "Plugin name to use for this connection",
			},
			"plugin_version": {
				Type:        framework.TypeString,
				Description: "Plugin version for this connection",
			},
			"verify_connection": {
				Type:        framework.TypeBool,
				Description: "Verify the database connection after saving",
				Default:     true,
			},
			"allowed_roles": {
				Type:        framework.TypeCommaStringSlice,
				Description: "Comma-separated list of roles allowed to use this connection",
			},
			"root_rotation_statements": {
				Type:        framework.TypeCommaStringSlice,
				Description: "Comma-separated list of statements to execute after root credential rotation",
			},
			"password_policy": {
				Type:        framework.TypeString,
				Description: "Password policy to use for generating passwords",
			},
			"connection_url": {
				Type:        framework.TypeString,
				Description: "Database connection URL",
			},
			"connection_string": {
				Type:        framework.TypeString,
				Description: "ODBC connection string for Teradata",
			},
			"username": {
				Type:        framework.TypeString,
				Description: "Database username",
			},
			"password": {
				Type:        framework.TypeString,
				Description: "Database password",
			},
			"disable_escaping": {
				Type:        framework.TypeBool,
				Description: "Disable special character escaping in passwords",
				Default:     false,
			},
			"min_connections": {
				Type:        framework.TypeInt,
				Description: "Minimum number of connections to maintain in the pool",
				Default:     0,
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
			"session_timeout": {
				Type:        framework.TypeInt,
				Description: "Session timeout in seconds (database-side limit for idle sessions)",
			},
			"max_connection_lifetime": {
				Type:        framework.TypeInt,
				Description: "Maximum connection lifetime in seconds (0 = no limit)",
				Default:     3600,
			},
			"idle_timeout": {
				Type:        framework.TypeInt,
				Description: "Idle connection timeout in seconds",
				Default:     300,
			},
			"ssl_mode": {
				Type:        framework.TypeString,
				Description: "SSL mode: disable, allow, verify-ca, verify-full, require",
				Default:     "disable",
			},
			"ssl_cert": {
				Type:        framework.TypeString,
				Description: "Path to SSL certificate file",
			},
			"ssl_key": {
				Type:        framework.TypeString,
				Description: "Path to SSL key file",
			},
			"ssl_root_cert": {
				Type:        framework.TypeString,
				Description: "Path to SSL root CA certificate file",
			},
			"ssl_key_password": {
				Type:        framework.TypeString,
				Description: "Password for the SSL key file",
			},
			"ssl_cipher_suites": {
				Type:        framework.TypeString,
				Description: "Comma-separated list of SSL cipher suites",
			},
			"ssl_secure": {
				Type:        framework.TypeBool,
				Description: "Enable SSL/TLS encryption",
				Default:     false,
			},
			"ssl_version": {
				Type:        framework.TypeString,
				Description: "SSL/TLS version (TLS 1.2, TLS 1.3)",
			},
			"max_retries": {
				Type:        framework.TypeInt,
				Description: "Maximum number of connection retry attempts",
				Default:     3,
			},
			"initial_retry_interval": {
				Type:        framework.TypeInt,
				Description: "Initial retry interval in milliseconds",
				Default:     100,
			},
			"max_retry_interval": {
				Type:        framework.TypeInt,
				Description: "Maximum retry interval in milliseconds",
				Default:     5000,
			},
			"retry_multiplier": {
				Type:        framework.TypeFloat,
				Description: "Exponential backoff multiplier for retries",
				Default:     2.0,
			},
			"session_variables": {
				Type:        framework.TypeMap,
				Description: "Session variables to set for connections (map of key-value pairs)",
			},
			"graceful_degradation_mode": {
				Type:        framework.TypeBool,
				Description: "Enable graceful degradation mode to continue operating when database is unavailable",
				Default:     false,
			},
			"max_result_rows": {
				Type:        framework.TypeInt,
				Description: "Maximum number of rows to return from query results (0 = no limit)",
				Default:     0,
			},
			"eviction_policy": {
				Type:        framework.TypeString,
				Description: "Connection eviction policy: lifo (newest first) or fifo (oldest first)",
				Default:     "lifo",
			},
			"eviction_batch_size": {
				Type:        framework.TypeInt,
				Description: "Number of connections to evict per cleanup cycle",
				Default:     1,
			},
			"eviction_grace_period": {
				Type:        framework.TypeInt,
				Description: "Additional grace period before forcing eviction (seconds)",
				Default:     30,
			},
			"min_evictable_idle_time": {
				Type:        framework.TypeInt,
				Description: "Minimum idle time before connection becomes evictable (seconds)",
				Default:     300,
			},
			"timezone": {
				Type:        framework.TypeString,
				Description: "Database time zone for the connection (e.g., 'America/New_York', 'UTC', '-08:00')",
			},
			"character_set": {
				Type:        framework.TypeString,
				Description: "Database character set for the connection (e.g., 'utf8', 'latin1', 'ascii')",
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

func (b *Backend) pathConfigList() *framework.Path {
	return &framework.Path{
		Pattern:         "config",
		HelpSynopsis:    "List all Teradata connections",
		HelpDescription: "Lists all configured Teradata database connections.",

		Operations: map[logical.Operation]framework.OperationHandler{
			logical.ListOperation: &framework.PathOperation{
				Callback: b.pathConfigListHandler,
			},
		},
	}
}

func (b *Backend) pathConfigReset() *framework.Path {
	return &framework.Path{
		Pattern:         "reset/" + framework.GenericNameRegex("name"),
		HelpSynopsis:    "Reset a Teradata connection",
		HelpDescription: "Resets and reinitializes a Teradata database connection.",

		Fields: map[string]*framework.FieldSchema{
			"name": {
				Type:        framework.TypeString,
				Description: "Name of the connection to reset",
			},
		},

		Operations: map[logical.Operation]framework.OperationHandler{
			logical.UpdateOperation: &framework.PathOperation{
				Callback: b.pathConfigResetHandler,
			},
		},
	}
}

func (b *Backend) pathConfigReload() *framework.Path {
	return &framework.Path{
		Pattern:         "reload/" + framework.GenericNameRegex("plugin_name"),
		HelpSynopsis:    "Reload connections for a plugin",
		HelpDescription: "Reloads all database connections for a specific plugin.",

		Fields: map[string]*framework.FieldSchema{
			"plugin_name": {
				Type:        framework.TypeString,
				Description: "Plugin name to reload connections for",
			},
		},

		Operations: map[logical.Operation]framework.OperationHandler{
			logical.UpdateOperation: &framework.PathOperation{
				Callback: b.pathConfigReloadHandler,
			},
		},
	}
}

func (b *Backend) pathConfigWrite(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	name := data.Get("name").(string)

	pluginName := data.Get("plugin_name").(string)
	pluginVersion := data.Get("plugin_version").(string)
	verifyConnection := data.Get("verify_connection").(bool)
	allowedRoles := data.Get("allowed_roles").([]string)
	rootRotationStatements := data.Get("root_rotation_statements").([]string)
	passwordPolicy := data.Get("password_policy").(string)
	connectionURL := data.Get("connection_url").(string)
	connectionString := data.Get("connection_string").(string)
	username := data.Get("username").(string)
	password := data.Get("password").(string)
	disableEscaping := data.Get("disable_escaping").(bool)

	if connectionString == "" && connectionURL == "" {
		return nil, fmt.Errorf("connection_string or connection_url is required")
	}

	if connectionString != "" {
		if err := security.ValidateConnectionString(connectionString); err != nil {
			return nil, fmt.Errorf("invalid connection_string: %w", err)
		}
	}

	minConnections := data.Get("min_connections").(int)
	maxOpenConnections := data.Get("max_open_connections").(int)
	maxIdleConnections := data.Get("max_idle_connections").(int)
	connectionTimeout := data.Get("connection_timeout").(int)
	sessionTimeout := data.Get("session_timeout").(int)
	maxConnectionLifetime := data.Get("max_connection_lifetime").(int)
	idleTimeout := data.Get("idle_timeout").(int)
	sslMode := data.Get("ssl_mode").(string)
	sslCert := data.Get("ssl_cert").(string)
	sslKey := data.Get("ssl_key").(string)
	sslRootCert := data.Get("ssl_root_cert").(string)
	sslKeyPassword := data.Get("ssl_key_password").(string)
	sslCipherSuites := data.Get("ssl_cipher_suites").(string)
	sslSecure := data.Get("ssl_secure").(bool)
	sslVersion := data.Get("ssl_version").(string)
	maxRetries := data.Get("max_retries").(int)
	initialRetryInterval := data.Get("initial_retry_interval").(int)
	maxRetryInterval := data.Get("max_retry_interval").(int)
	retryMultiplier := data.Get("retry_multiplier").(float64)
	gracefulDegradationMode := data.Get("graceful_degradation_mode").(bool)
	maxResultRows := data.Get("max_result_rows").(int)
	evictionPolicy := data.Get("eviction_policy").(string)
	evictionBatchSize := data.Get("eviction_batch_size").(int)
	evictionGracePeriod := data.Get("eviction_grace_period").(int)
	minEvictableIdleTime := data.Get("min_evictable_idle_time").(int)
	timeZone := data.Get("timezone").(string)
	characterSet := data.Get("character_set").(string)

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

	if minConnections < 0 {
		return nil, fmt.Errorf("min_connections cannot be negative")
	}
	if maxOpenConnections < minConnections {
		return nil, fmt.Errorf("max_open_connections must be >= min_connections")
	}
	if maxIdleConnections > maxOpenConnections {
		return nil, fmt.Errorf("max_idle_connections cannot exceed max_open_connections")
	}

	validSSLModes := map[string]bool{
		"disable":     true,
		"allow":       true,
		"verify-ca":   true,
		"verify-full": true,
		"require":     true,
	}
	if sslMode != "" && !validSSLModes[sslMode] {
		return nil, fmt.Errorf("invalid ssl_mode: %s, must be one of: disable, allow, verify-ca, verify-full, require", sslMode)
	}

	if maxRetries < 0 {
		return nil, fmt.Errorf("max_retries cannot be negative")
	}
	if initialRetryInterval < 0 {
		return nil, fmt.Errorf("initial_retry_interval cannot be negative")
	}
	if maxRetryInterval < 0 {
		return nil, fmt.Errorf("max_retry_interval cannot be negative")
	}
	if retryMultiplier < 0 {
		return nil, fmt.Errorf("retry_multiplier cannot be negative")
	}
	if maxResultRows < 0 {
		return nil, fmt.Errorf("max_result_rows cannot be negative")
	}
	if evictionPolicy != "" && evictionPolicy != "lifo" && evictionPolicy != "fifo" {
		return nil, fmt.Errorf("eviction_policy must be 'lifo' or 'fifo'")
	}
	if evictionBatchSize < 0 {
		return nil, fmt.Errorf("eviction_batch_size cannot be negative")
	}
	if evictionGracePeriod < 0 {
		return nil, fmt.Errorf("eviction_grace_period cannot be negative")
	}
	if minEvictableIdleTime < 0 {
		return nil, fmt.Errorf("min_evictable_idle_time cannot be negative")
	}

	cfg := &models.Config{
		Name:                    name,
		PluginName:              pluginName,
		PluginVersion:           pluginVersion,
		VerifyConnection:        verifyConnection,
		AllowedRoles:            allowedRoles,
		RootRotationStatements:  rootRotationStatements,
		PasswordPolicy:          passwordPolicy,
		ConnectionURL:           connectionURL,
		ConnectionString:        connectionString,
		Username:                username,
		Password:                password,
		DisableEscaping:         disableEscaping,
		MinConnections:          minConnections,
		MaxOpenConnections:      maxOpenConnections,
		MaxIdleConnections:      maxIdleConnections,
		ConnectionTimeout:       connectionTimeout,
		SessionTimeout:          sessionTimeout,
		MaxConnectionLifetime:   maxConnectionLifetime,
		IdleTimeout:             idleTimeout,
		SSLMode:                 sslMode,
		SSLCert:                 sslCert,
		SSLKey:                  sslKey,
		SSLRootCert:             sslRootCert,
		SSLKeyPassword:          sslKeyPassword,
		SSLCipherSuites:         sslCipherSuites,
		SSLSecure:               sslSecure,
		SSLVersion:              sslVersion,
		SessionVariables:        sessionVariables,
		MaxRetries:              maxRetries,
		InitialRetryInterval:    initialRetryInterval,
		MaxRetryInterval:        maxRetryInterval,
		RetryMultiplier:         retryMultiplier,
		GracefulDegradationMode: gracefulDegradationMode,
		MaxResultRows:           maxResultRows,
		EvictionPolicy:          evictionPolicy,
		EvictionBatchSize:       evictionBatchSize,
		EvictionGracePeriod:     evictionGracePeriod,
		MinEvictableIdleTime:    minEvictableIdleTime,
		TimeZone:                timeZone,
		CharacterSet:            characterSet,
		Version:                 1,
	}

	storageKey := "config/" + name

	entry, err := logical.StorageEntryJSON(storageKey, cfg)
	if err != nil {
		return nil, err
	}

	if err := req.Storage.Put(ctx, entry); err != nil {
		return nil, err
	}

	b.invalidateConfigCache(name)

	if verifyConnection {
		if err := b.verifyConfigConnection(ctx, cfg); err != nil {
			b.invalidateConfigCache(name)
			return nil, fmt.Errorf("connection verification failed: %w", err)
		}
	}

	return b.pathConfigRead(ctx, req, data)
}

func (b *Backend) verifyConfigConnection(ctx context.Context, cfg *models.Config) error {
	connString, err := buildConnectionString(cfg)
	if err != nil {
		return fmt.Errorf("failed to build connection string: %w", err)
	}

	conn, err := teradb.Connect(connString)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer conn.Close()

	return nil
}

func (b *Backend) pathConfigRead(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	name := data.Get("name").(string)

	if name == "" {
		return nil, fmt.Errorf("name is required")
	}

	storageKey := "config/" + name

	entry, err := req.Storage.Get(ctx, storageKey)
	if err != nil {
		return nil, err
	}

	if entry == nil {
		return nil, nil
	}

	var cfg models.Config
	if err := entry.DecodeJSON(&cfg); err != nil {
		return nil, err
	}

	connectionDetails := map[string]interface{}{
		"connection_string":         "***",
		"connection_url":            cfg.ConnectionURL,
		"username":                  cfg.Username,
		"disable_escaping":          cfg.DisableEscaping,
		"min_connections":           cfg.MinConnections,
		"max_open_connections":      cfg.MaxOpenConnections,
		"max_idle_connections":      cfg.MaxIdleConnections,
		"connection_timeout":        cfg.ConnectionTimeout,
		"session_timeout":           cfg.SessionTimeout,
		"max_connection_lifetime":   cfg.MaxConnectionLifetime,
		"idle_timeout":              cfg.IdleTimeout,
		"ssl_mode":                  cfg.SSLMode,
		"ssl_cert":                  cfg.SSLCert,
		"ssl_key":                   cfg.SSLKey,
		"ssl_root_cert":             cfg.SSLRootCert,
		"ssl_cipher_suites":         cfg.SSLCipherSuites,
		"ssl_secure":                cfg.SSLSecure,
		"ssl_version":               cfg.SSLVersion,
		"session_variables":         cfg.SessionVariables,
		"max_retries":               cfg.MaxRetries,
		"initial_retry_interval":    cfg.InitialRetryInterval,
		"max_retry_interval":        cfg.MaxRetryInterval,
		"retry_multiplier":          cfg.RetryMultiplier,
		"graceful_degradation_mode": cfg.GracefulDegradationMode,
		"max_result_rows":           cfg.MaxResultRows,
		"eviction_policy":           cfg.EvictionPolicy,
		"eviction_batch_size":       cfg.EvictionBatchSize,
		"eviction_grace_period":     cfg.EvictionGracePeriod,
		"min_evictable_idle_time":   cfg.MinEvictableIdleTime,
		"timezone":                  cfg.TimeZone,
		"character_set":             cfg.CharacterSet,
	}

	if cfg.Password != "" {
		connectionDetails["password"] = "***"
	}
	if cfg.SSLKeyPassword != "" {
		connectionDetails["ssl_key_password"] = "***"
	}

	respData := map[string]interface{}{
		"name":                     name,
		"plugin_name":              cfg.PluginName,
		"plugin_version":           cfg.PluginVersion,
		"verify_connection":        cfg.VerifyConnection,
		"allowed_roles":            cfg.AllowedRoles,
		"root_rotation_statements": cfg.RootRotationStatements,
		"password_policy":          cfg.PasswordPolicy,
		"connection_details":       connectionDetails,
	}

	return &logical.Response{
		Data: respData,
	}, nil
}

func (b *Backend) pathConfigDelete(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	name := data.Get("name").(string)

	if name == "" {
		return nil, fmt.Errorf("name is required")
	}

	storageKey := "config/" + name

	err := req.Storage.Delete(ctx, storageKey)
	if err != nil {
		return nil, fmt.Errorf("error deleting config: %w", err)
	}

	b.invalidateConfigCache(name)

	return nil, nil
}

func (b *Backend) pathConfigListHandler(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	entries, err := req.Storage.List(ctx, "config/")
	if err != nil {
		return nil, err
	}

	return logical.ListResponse(entries), nil
}

func (b *Backend) pathConfigResetHandler(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	name := data.Get("name").(string)

	if name == "" {
		return nil, fmt.Errorf("name is required")
	}

	storageKey := "config/" + name

	entry, err := req.Storage.Get(ctx, storageKey)
	if err != nil {
		return nil, err
	}

	if entry == nil {
		return nil, fmt.Errorf("configuration %q not found", name)
	}

	var cfg models.Config
	if err := entry.DecodeJSON(&cfg); err != nil {
		return nil, err
	}

	dbConfig := &storage.DBConfig{
		Name:                  name,
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

	_, err = b.dbRegistry.UpdateConnection(name, dbConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to reset connection: %w", err)
	}

	return &logical.Response{
		Data: map[string]interface{}{
			"message": fmt.Sprintf("connection %q reset successfully", name),
		},
	}, nil
}

func (b *Backend) pathConfigReloadHandler(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	pluginName := data.Get("plugin_name").(string)

	if pluginName == "" {
		return nil, fmt.Errorf("plugin_name is required")
	}

	entries, err := req.Storage.List(ctx, "config/")
	if err != nil {
		return nil, err
	}

	reloadedCount := 0
	for _, entry := range entries {
		storageKey := "config/" + entry
		cfgEntry, err := req.Storage.Get(ctx, storageKey)
		if err != nil {
			continue
		}
		if cfgEntry == nil {
			continue
		}

		var cfg models.Config
		if err := cfgEntry.DecodeJSON(&cfg); err != nil {
			continue
		}

		if cfg.PluginName != pluginName {
			continue
		}

		dbConfig := &storage.DBConfig{
			Name:                  entry,
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

		_, err = b.dbRegistry.UpdateConnection(entry, dbConfig)
		if err == nil {
			reloadedCount++
		}
	}

	return &logical.Response{
		Data: map[string]interface{}{
			"message":        fmt.Sprintf("reloaded %d connections for plugin %q", reloadedCount, pluginName),
			"reloaded_count": reloadedCount,
			"plugin_name":    pluginName,
		},
	}, nil
}

func getConfig(ctx context.Context, storage logical.Storage) (*models.Config, error) {
	entry, err := storage.Get(ctx, "config")
	if err != nil {
		return nil, err
	}

	if entry == nil {
		return nil, nil
	}

	var cfg models.Config
	if err := entry.DecodeJSON(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func getConfigByName(ctx context.Context, storage logical.Storage, name string) (*models.Config, error) {
	if name == "" {
		return getConfig(ctx, storage)
	}
	entry, err := storage.Get(ctx, "config/"+name)
	if err != nil {
		return nil, err
	}

	if entry == nil {
		return nil, nil
	}

	var cfg models.Config
	if err := entry.DecodeJSON(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}
