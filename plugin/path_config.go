package teradata

import (
	"context"
	"fmt"

	"github.com/JavierLimon/openbao-teradata-secret-plugin/models"
	"github.com/openbao/openbao/sdk/v2/framework"
	"github.com/openbao/openbao/sdk/v2/logical"
)

func (b *Backend) pathConfig() *framework.Path {
	return &framework.Path{
		Pattern:         "config",
		HelpSynopsis:    "Configure the Teradata connection",
		HelpDescription: "Configures the connection parameters for the Teradata database.",

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

func (b *Backend) pathConfigWrite(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	connectionString := data.Get("connection_string").(string)
	maxOpenConnections := data.Get("max_open_connections").(int)
	maxIdleConnections := data.Get("max_idle_connections").(int)
	connectionTimeout := data.Get("connection_timeout").(int)

	cfg := &models.Config{
		ConnectionString:   connectionString,
		MaxOpenConnections: maxOpenConnections,
		MaxIdleConnections: maxIdleConnections,
		ConnectionTimeout:  connectionTimeout,
	}

	entry, err := logical.StorageEntryJSON("config", cfg)
	if err != nil {
		return nil, err
	}

	if err := req.Storage.Put(ctx, entry); err != nil {
		return nil, err
	}

	return &logical.Response{
		Data: map[string]interface{}{
			"connection_string":    "***",
			"max_open_connections": maxOpenConnections,
			"max_idle_connections": maxIdleConnections,
			"connection_timeout":   connectionTimeout,
		},
	}, nil
}

func (b *Backend) pathConfigRead(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	entry, err := req.Storage.Get(ctx, "config")
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

	return &logical.Response{
		Data: map[string]interface{}{
			"connection_string":    "***",
			"max_open_connections": cfg.MaxOpenConnections,
			"max_idle_connections": cfg.MaxIdleConnections,
			"connection_timeout":   cfg.ConnectionTimeout,
		},
	}, nil
}

func (b *Backend) pathConfigDelete(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	err := req.Storage.Delete(ctx, "config")
	if err != nil {
		return nil, fmt.Errorf("error deleting config: %w", err)
	}

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
