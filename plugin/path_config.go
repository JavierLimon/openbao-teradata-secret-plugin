package teradata

import (
	"context"
	"fmt"

	"github.com/JavierLimon/openbao-teradata-secret-plugin/models"
	"github.com/JavierLimon/openbao-teradata-secret-plugin/security"
	"github.com/openbao/openbao/sdk/v2/framework"
	"github.com/openbao/openbao/sdk/v2/logical"
)

func (b *Backend) pathConfig() *framework.Path {
	return &framework.Path{
		Pattern:         "config/(?P<region>[a-zA-Z0-9_-]+)",
		HelpSynopsis:    "Configure the Teradata connection for a specific region",
		HelpDescription: "Configures the connection parameters for a specific Teradata database region.",

		Fields: map[string]*framework.FieldSchema{
			"region": {
				Type:        framework.TypeString,
				Description: "Region identifier",
			},
			"connection_string": {
				Type:        framework.TypeString,
				Description: "ODBC connection string for Teradata",
				Required:    true,
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

func (b *Backend) pathConfigWrite(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	var region string
	if r, ok := data.Raw["region"].(string); ok {
		region = r
	}
	connectionString := data.Get("connection_string").(string)

	if err := security.ValidateConnectionString(connectionString); err != nil {
		return nil, fmt.Errorf("invalid connection string: %w", err)
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
		Region:                  region,
		ConnectionString:        connectionString,
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
	}

	storageKey := "config"
	if region != "" {
		storageKey = "config/" + region
	}

	entry, err := logical.StorageEntryJSON(storageKey, cfg)
	if err != nil {
		return nil, err
	}

	if err := req.Storage.Put(ctx, entry); err != nil {
		return nil, err
	}

	b.invalidateConfigCache(region)

	respData := map[string]interface{}{
		"connection_string":         "***",
		"min_connections":           minConnections,
		"max_open_connections":      maxOpenConnections,
		"max_idle_connections":      maxIdleConnections,
		"connection_timeout":        connectionTimeout,
		"session_timeout":           sessionTimeout,
		"max_connection_lifetime":   maxConnectionLifetime,
		"idle_timeout":              idleTimeout,
		"ssl_mode":                  sslMode,
		"ssl_cert":                  sslCert,
		"ssl_key":                   sslKey,
		"ssl_root_cert":             sslRootCert,
		"ssl_cipher_suites":         sslCipherSuites,
		"ssl_secure":                sslSecure,
		"ssl_version":               sslVersion,
		"max_retries":               maxRetries,
		"initial_retry_interval":    initialRetryInterval,
		"max_retry_interval":        maxRetryInterval,
		"retry_multiplier":          retryMultiplier,
		"graceful_degradation_mode": gracefulDegradationMode,
		"max_result_rows":           maxResultRows,
		"eviction_policy":           evictionPolicy,
		"eviction_batch_size":       evictionBatchSize,
		"eviction_grace_period":     evictionGracePeriod,
		"min_evictable_idle_time":   minEvictableIdleTime,
	}
	if region != "" {
		respData["region"] = region
	}
	if sslKeyPassword != "" {
		respData["ssl_key_password"] = "***"
	}

	return &logical.Response{
		Data: respData,
	}, nil
}

func (b *Backend) pathConfigRead(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	var region string
	if r, ok := data.Raw["region"].(string); ok {
		region = r
	}

	storageKey := "config"
	if region != "" {
		storageKey = "config/" + region
	}

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

	respData := map[string]interface{}{
		"connection_string":         "***",
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
	}
	if cfg.Region != "" {
		respData["region"] = cfg.Region
	}
	if cfg.SSLKeyPassword != "" {
		respData["ssl_key_password"] = "***"
	}

	return &logical.Response{
		Data: respData,
	}, nil
}

func (b *Backend) pathConfigDelete(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	var region string
	if r, ok := data.Raw["region"].(string); ok {
		region = r
	}

	storageKey := "config"
	if region != "" {
		storageKey = "config/" + region
	}

	err := req.Storage.Delete(ctx, storageKey)
	if err != nil {
		return nil, fmt.Errorf("error deleting config: %w", err)
	}

	b.invalidateConfigCache(region)

	return nil, nil
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

func getConfigByRegion(ctx context.Context, storage logical.Storage, region string) (*models.Config, error) {
	if region == "" {
		return getConfig(ctx, storage)
	}
	entry, err := storage.Get(ctx, "config/"+region)
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
