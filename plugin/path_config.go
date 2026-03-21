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

	if minConnections < 0 {
		return nil, fmt.Errorf("min_connections cannot be negative")
	}
	if maxOpenConnections < minConnections {
		return nil, fmt.Errorf("max_open_connections must be >= min_connections")
	}
	if maxIdleConnections > maxOpenConnections {
		return nil, fmt.Errorf("max_idle_connections cannot exceed max_open_connections")
	}

	cfg := &models.Config{
		Region:             region,
		ConnectionString:   connectionString,
		MinConnections:     minConnections,
		MaxOpenConnections: maxOpenConnections,
		MaxIdleConnections: maxIdleConnections,
		ConnectionTimeout:  connectionTimeout,
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
		"connection_string":    "***",
		"min_connections":      minConnections,
		"max_open_connections": maxOpenConnections,
		"max_idle_connections": maxIdleConnections,
		"connection_timeout":   connectionTimeout,
	}
	if region != "" {
		respData["region"] = region
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
		"connection_string":    "***",
		"min_connections":      cfg.MinConnections,
		"max_open_connections": cfg.MaxOpenConnections,
		"max_idle_connections": cfg.MaxIdleConnections,
		"connection_timeout":   cfg.ConnectionTimeout,
	}
	if cfg.Region != "" {
		respData["region"] = cfg.Region
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
