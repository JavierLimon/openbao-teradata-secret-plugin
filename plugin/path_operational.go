package teradata

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/JavierLimon/openbao-teradata-secret-plugin/logging"
	"github.com/JavierLimon/openbao-teradata-secret-plugin/odbc"
	"github.com/openbao/openbao/sdk/v2/framework"
	"github.com/openbao/openbao/sdk/v2/logical"
)

func (b *Backend) pathHealth() *framework.Path {
	return &framework.Path{
		Pattern:         "health",
		HelpSynopsis:    "Health check",
		HelpDescription: "Returns the health status of the Teradata plugin.",

		Operations: map[logical.Operation]framework.OperationHandler{
			logical.ReadOperation: &framework.PathOperation{
				Callback: b.pathHealthRead,
			},
		},
	}
}

func (b *Backend) pathHealthRead(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	cfg, err := getConfig(ctx, req.Storage)
	if err != nil {
		logging.Default().Warn("health_check_failed",
			slog.String("reason", "config_error"),
			slog.String("error", err.Error()),
			slog.Time("timestamp", time.Now()),
		)
		return &logical.Response{
			Data: map[string]interface{}{
				"status":      "unhealthy",
				"initialized": false,
				"error":       err.Error(),
			},
		}, nil
	}

	if cfg == nil {
		return &logical.Response{
			Data: map[string]interface{}{
				"status":      "uninitialized",
				"initialized": false,
			},
		}, nil
	}

	registry := b.getDBRegistry()
	connectionNames := registry.ListConnections()

	poolStatus := make(map[string]interface{})
	overallHealthy := true

	for _, name := range connectionNames {
		state, openConns, idleConns, connErr := registry.GetConnectionStats(name)
		conn, _ := registry.GetConnection(name)

		var lastHealthCheck time.Time
		if conn != nil {
			lastHealthCheck = conn.LastHealthCheck()
		}

		poolInfo := map[string]interface{}{
			"state":             stateToString(state),
			"open_connections":  openConns,
			"idle_connections":  idleConns,
			"in_use":            openConns - idleConns,
			"last_health_check": lastHealthCheck,
		}

		if connErr != nil {
			poolInfo["error"] = connErr.Error()
			overallHealthy = false
			logging.LogConnectionEvent(nil, "health_check_failed", name, map[string]interface{}{
				"error": connErr.Error(),
			})
		}

		poolStatus[name] = poolInfo
	}

	status := "healthy"
	if !overallHealthy {
		status = "degraded"
	}
	if len(connectionNames) == 0 {
		status = "configured_no_connections"
	}

	logging.Default().Info("health_check_completed",
		slog.String("status", status),
		slog.Int("pool_count", len(connectionNames)),
		slog.Time("timestamp", time.Now()),
	)

	return &logical.Response{
		Data: map[string]interface{}{
			"status":      status,
			"initialized": true,
			"pool_count":  len(connectionNames),
			"pool_status": poolStatus,
			"checked_at":  time.Now(),
		},
	}, nil
}

func (b *Backend) pathVersion() *framework.Path {
	return &framework.Path{
		Pattern:         "version",
		HelpSynopsis:    "Plugin version",
		HelpDescription: "Returns the version of the Teradata plugin.",

		Operations: map[logical.Operation]framework.OperationHandler{
			logical.ReadOperation: &framework.PathOperation{
				Callback: b.pathVersionRead,
			},
		},
	}
}

func (b *Backend) pathVersionRead(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	return &logical.Response{
		Data: map[string]interface{}{
			"version": Version,
		},
	}, nil
}

func (b *Backend) pathAPIVersion() *framework.Path {
	return &framework.Path{
		Pattern:         "api-version",
		HelpSynopsis:    "API version information",
		HelpDescription: "Returns the API version information for the Teradata plugin.",

		Operations: map[logical.Operation]framework.OperationHandler{
			logical.ReadOperation: &framework.PathOperation{
				Callback: b.pathAPIVersionRead,
			},
		},
	}
}

func (b *Backend) pathAPIVersionRead(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	return &logical.Response{
		Data: map[string]interface{}{
			"api_version":         APIVersion,
			"api_version_major":   APIVersionMajor,
			"api_version_minor":   APIVersionMinor,
			"plugin_version":      Version,
			"min_supported_major": MinSupportedMajor,
			"min_supported_minor": MinSupportedMinor,
		},
	}, nil
}

func (b *Backend) pathReadiness() *framework.Path {
	return &framework.Path{
		Pattern:         "readiness",
		HelpSynopsis:    "Readiness probe",
		HelpDescription: "Returns the readiness status of the Teradata plugin. Used by Kubernetes to determine if the plugin is ready to serve traffic.",

		Operations: map[logical.Operation]framework.OperationHandler{
			logical.ReadOperation: &framework.PathOperation{
				Callback: b.pathReadinessRead,
			},
		},
	}
}

func (b *Backend) pathReadinessRead(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	cfg, err := getConfig(ctx, req.Storage)
	if err != nil {
		return &logical.Response{
			Data: map[string]interface{}{
				"status":      "not ready",
				"ready":       false,
				"initialized": false,
				"error":       err.Error(),
			},
		}, nil
	}

	if cfg == nil {
		return &logical.Response{
			Data: map[string]interface{}{
				"status":      "not ready",
				"ready":       false,
				"initialized": false,
			},
		}, nil
	}

	conn, err := odbc.Connect(odbc.AppendQueryTimeout(cfg.ConnectionString, cfg.QueryTimeout))
	if err != nil {
		return &logical.Response{
			Data: map[string]interface{}{
				"status":      "not ready",
				"ready":       false,
				"initialized": true,
				"error":       fmt.Sprintf("database connection failed: %s", err.Error()),
			},
		}, nil
	}
	defer conn.Close()

	if err := conn.Ping(); err != nil {
		return &logical.Response{
			Data: map[string]interface{}{
				"status":      "not ready",
				"ready":       false,
				"initialized": true,
				"error":       fmt.Sprintf("database ping failed: %s", err.Error()),
			},
		}, nil
	}

	return &logical.Response{
		Data: map[string]interface{}{
			"status":      "ready",
			"ready":       true,
			"initialized": true,
		},
	}, nil
}

func (b *Backend) pathLiveness() *framework.Path {
	return &framework.Path{
		Pattern:         "liveness",
		HelpSynopsis:    "Liveness probe",
		HelpDescription: "Returns the liveness status of the Teradata plugin. Used by Kubernetes to determine if the plugin is alive.",

		Operations: map[logical.Operation]framework.OperationHandler{
			logical.ReadOperation: &framework.PathOperation{
				Callback: b.pathLivenessRead,
			},
		},
	}
}

func (b *Backend) pathLivenessRead(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	return &logical.Response{
		Data: map[string]interface{}{
			"status":  "alive",
			"alive":   true,
			"version": Version,
		},
	}, nil
}

func (b *Backend) pathInfo() *framework.Path {
	return &framework.Path{
		Pattern:         "info",
		HelpSynopsis:    "Database and driver information",
		HelpDescription: "Returns database driver info and Teradata version information.",

		Operations: map[logical.Operation]framework.OperationHandler{
			logical.ReadOperation: &framework.PathOperation{
				Callback: b.pathInfoRead,
			},
		},
	}
}

func (b *Backend) pathInfoRead(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	cfg, err := getConfig(ctx, req.Storage)
	if err != nil {
		return nil, fmt.Errorf("failed to get config: %w", err)
	}

	if cfg == nil {
		return &logical.Response{
			Data: map[string]interface{}{
				"status":      "unconfigured",
				"initialized": false,
			},
		}, nil
	}

	conn, err := odbc.Connect(odbc.AppendQueryTimeout(cfg.ConnectionString, cfg.QueryTimeout))
	if err != nil {
		return &logical.Response{
			Data: map[string]interface{}{
				"status":      "disconnected",
				"initialized": true,
				"error":       fmt.Sprintf("database connection failed: %s", err.Error()),
			},
		}, nil
	}
	defer conn.Close()

	dbInfo, err := conn.GetDatabaseInfo(ctx)
	if err != nil {
		return &logical.Response{
			Data: map[string]interface{}{
				"status":      "error",
				"initialized": true,
				"error":       fmt.Sprintf("failed to get database info: %s", err.Error()),
			},
		}, nil
	}

	return &logical.Response{
		Data: map[string]interface{}{
			"status":         "connected",
			"initialized":    true,
			"driver_name":    dbInfo.DriverName,
			"driver_version": dbInfo.DriverVersion,
			"db_version":     dbInfo.DBVersion,
			"db_name":        dbInfo.DBName,
			"plugin_version": Version,
		},
	}, nil
}
