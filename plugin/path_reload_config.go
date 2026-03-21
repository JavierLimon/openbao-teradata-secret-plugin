package teradata

import (
	"context"
	"fmt"
	"time"

	"github.com/JavierLimon/openbao-teradata-secret-plugin/models"
	"github.com/JavierLimon/openbao-teradata-secret-plugin/storage"
	"github.com/openbao/openbao/sdk/v2/framework"
	"github.com/openbao/openbao/sdk/v2/logical"
)

func (b *Backend) pathReloadConfig() *framework.Path {
	return &framework.Path{
		Pattern:         "reload-config",
		HelpSynopsis:    "Reload plugin configuration",
		HelpDescription: "Reloads the plugin configuration and reconnects to the database using the current configuration.",

		Fields: map[string]*framework.FieldSchema{
			"region": {
				Type:        framework.TypeString,
				Description: "Region identifier to reload (omit for default config)",
			},
		},

		Operations: map[logical.Operation]framework.OperationHandler{
			logical.UpdateOperation: &framework.PathOperation{
				Callback: b.pathReloadConfigHandler,
			},
			logical.ReadOperation: &framework.PathOperation{
				Callback: b.pathReloadConfigHandler,
			},
		},
	}
}

func (b *Backend) pathReloadConfigHandler(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	var region string
	if r, ok := data.Raw["region"].(string); ok {
		region = r
	}

	storageKey := "config"
	connectionName := "default"
	if region != "" {
		storageKey = "config/" + region
		connectionName = region
	}

	entry, err := req.Storage.Get(ctx, storageKey)
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	if entry == nil {
		return &logical.Response{
			Data: map[string]interface{}{
				"success": false,
				"message": fmt.Sprintf("configuration for region %q not found", region),
			},
		}, nil
	}

	var cfg models.Config
	if err := entry.DecodeJSON(&cfg); err != nil {
		return nil, fmt.Errorf("failed to decode config: %w", err)
	}

	b.invalidateConfigCache(region)

	registry := b.getDBRegistry()

	dbConfig := &storage.DBConfig{
		Name:                  connectionName,
		ConnectionString:      cfg.ConnectionString,
		MinConnections:        cfg.MinConnections,
		MaxOpenConnections:    cfg.MaxOpenConnections,
		MaxIdleConnections:    cfg.MaxIdleConnections,
		ConnectionTimeout:     time.Duration(cfg.ConnectionTimeout) * time.Second,
		MaxConnectionLifetime: time.Duration(cfg.MaxConnectionLifetime) * time.Second,
		IdleTimeout:           time.Duration(cfg.IdleTimeout) * time.Second,
		HealthCheckInterval:   30 * time.Second,
		HealthCheckTimeout:    5 * time.Second,
		SSLMode:               cfg.SSLMode,
		SSLCert:               cfg.SSLCert,
		SSLKey:                cfg.SSLKey,
		SSLRootCert:           cfg.SSLRootCert,
		SSLKeyPassword:        cfg.SSLKeyPassword,
		SSLCipherSuites:       cfg.SSLCipherSuites,
		SSLSecure:             cfg.SSLSecure,
		SSLVersion:            cfg.SSLVersion,
	}

	_, err = registry.UpdateConnection(connectionName, dbConfig)
	if err != nil {
		return &logical.Response{
			Data: map[string]interface{}{
				"success": false,
				"message": fmt.Sprintf("failed to update connection: %s", err.Error()),
			},
		}, nil
	}

	return &logical.Response{
		Data: map[string]interface{}{
			"success": true,
			"message": fmt.Sprintf("configuration for region %q reloaded successfully", region),
		},
	}, nil
}

func (b *Backend) pathReloadConfigV1() *framework.Path {
	return &framework.Path{
		Pattern:         "config/reload",
		HelpSynopsis:    "Reload plugin configuration (v1 compatibility)",
		HelpDescription: "Reloads the plugin configuration and reconnects to the database using the current configuration.",

		Fields: map[string]*framework.FieldSchema{
			"region": {
				Type:        framework.TypeString,
				Description: "Region identifier to reload (omit for default config)",
			},
		},

		Operations: map[logical.Operation]framework.OperationHandler{
			logical.UpdateOperation: &framework.PathOperation{
				Callback: b.pathReloadConfigHandler,
			},
			logical.ReadOperation: &framework.PathOperation{
				Callback: b.pathReloadConfigHandler,
			},
		},
	}
}
