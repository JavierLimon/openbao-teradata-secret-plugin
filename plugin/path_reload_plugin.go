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

func (b *Backend) pathReloadPlugin() *framework.Path {
	return &framework.Path{
		Pattern:         "reload/plugin",
		HelpSynopsis:    "Reload plugin configuration",
		HelpDescription: "Reloads all plugin-level settings including caches, connection pools, and health checks.",

		Operations: map[logical.Operation]framework.OperationHandler{
			logical.UpdateOperation: &framework.PathOperation{
				Callback: b.pathReloadPluginHandler,
			},
			logical.ReadOperation: &framework.PathOperation{
				Callback: b.pathReloadPluginHandler,
			},
		},
	}
}

func (b *Backend) pathReloadPluginHandler(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	var reloadedPools []string
	var failedPools []string

	configKeys := []string{"config"}

	entries, err := b.storage.List(ctx, "config/")
	if err != nil {
		return nil, fmt.Errorf("failed to list config keys: %w", err)
	}
	for _, entry := range entries {
		configKeys = append(configKeys, "config/"+entry)
	}

	if b.credCache != nil {
		b.credCache.Close()
		b.credCache = newCredentialCache(5*time.Minute, 10000)
	}

	if b.queryCache != nil {
		b.queryCache.Close()
		b.queryCache = newQueryResultCache(5*time.Minute, 1000)
	}

	if b.dbRegistry != nil {
		b.dbRegistry.Shutdown()
	}
	b.dbRegistry = storage.NewDBRegistry()
	b.dbRegistry.StartHealthChecks()

	for _, key := range configKeys {
		entry, err := b.storage.Get(ctx, key)
		if err != nil {
			failedPools = append(failedPools, key)
			continue
		}
		if entry == nil {
			continue
		}

		var cfg models.Config
		if err := entry.DecodeJSON(&cfg); err != nil {
			failedPools = append(failedPools, key)
			continue
		}

		if cfg.ConnectionString == "" {
			continue
		}

		dbConfig := &storage.DBConfig{
			Name:                  cfg.Region,
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

		name := cfg.Region
		if name == "" {
			name = "default"
		}

		conn, err := b.dbRegistry.AddConnection(name, dbConfig)
		if err != nil {
			failedPools = append(failedPools, name)
			continue
		}

		if conn != nil {
			conn.WaitForWarmup()
		}
		reloadedPools = append(reloadedPools, name)
	}

	response := &logical.Response{
		Data: map[string]interface{}{
			"success":             true,
			"message":             "plugin configuration reloaded successfully",
			"reloaded_pools":      reloadedPools,
			"reloaded_pool_count": len(reloadedPools),
		},
	}

	if len(failedPools) > 0 {
		response.Data["success"] = false
		response.Data["message"] = "plugin configuration reloaded with errors"
		response.Data["failed_pools"] = failedPools
	}

	return response, nil
}
