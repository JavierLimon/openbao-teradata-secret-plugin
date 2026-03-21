package teradata

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/JavierLimon/openbao-teradata-secret-plugin/logging"
	"github.com/JavierLimon/openbao-teradata-secret-plugin/odbc"
	"github.com/JavierLimon/openbao-teradata-secret-plugin/storage"
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
				"status":               "unhealthy",
				"initialized":          false,
				"error":                err.Error(),
				"graceful_degradation": b.IsDegraded(),
			},
		}, nil
	}

	if cfg == nil {
		return &logical.Response{
			Data: map[string]interface{}{
				"status":               "uninitialized",
				"initialized":          false,
				"graceful_degradation": b.IsDegraded(),
			},
		}, nil
	}

	if cfg.GracefulDegradationMode {
		b.SetGracefulDegradation(true)
	}

	registry := b.getDBRegistry()
	connectionNames := registry.ListConnections()

	poolStatus := make(map[string]interface{})
	overallHealthy := true
	degradedPools := []string{}
	healthyPools := []string{}

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
			degradedPools = append(degradedPools, name)
			logging.LogConnectionEvent(nil, "health_check_failed", name, map[string]interface{}{
				"error": connErr.Error(),
			})
		} else if state != storage.StateHealthy {
			overallHealthy = false
			degradedPools = append(degradedPools, name)
		} else {
			healthyPools = append(healthyPools, name)
		}

		poolStatus[name] = poolInfo
	}

	status := "healthy"
	if !overallHealthy {
		status = "degraded"
		b.SetGracefulDegradation(true)
	}
	if len(connectionNames) == 0 {
		status = "configured_no_connections"
	}

	if overallHealthy && len(connectionNames) > 0 {
		b.SetGracefulDegradation(false)
	}

	logging.Default().Info("health_check_completed",
		slog.String("status", status),
		slog.Int("pool_count", len(connectionNames)),
		slog.Time("timestamp", time.Now()),
	)

	respData := map[string]interface{}{
		"status":                       status,
		"initialized":                  true,
		"pool_count":                   len(connectionNames),
		"pool_status":                  poolStatus,
		"checked_at":                   time.Now(),
		"graceful_degradation":         b.IsDegraded(),
		"manually_enabled_degradation": b.IsGracefulDegradationManuallyEnabled(),
		"degraded_pools":               degradedPools,
		"healthy_pools":                healthyPools,
	}

	if b.IsDegraded() && !b.DegradedSince().IsZero() {
		respData["degraded_since"] = b.DegradedSince()
		respData["degradation_duration_seconds"] = time.Since(b.DegradedSince()).Seconds()
	}

	if cfg.GracefulDegradationMode {
		respData["graceful_degradation_mode_enabled"] = true
		respData["graceful_degradation_note"] = "Plugin is operating in graceful degradation mode. Credential operations may be limited."
	}

	return &logical.Response{
		Data: respData,
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

	conn, err := odbc.Connect(odbc.AppendSessionTimeout(odbc.AppendQueryTimeout(cfg.ConnectionString, cfg.QueryTimeout), cfg.SessionTimeout))
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

func (b *Backend) pathDegradation() *framework.Path {
	return &framework.Path{
		Pattern:         "degradation",
		HelpSynopsis:    "Graceful degradation control",
		HelpDescription: "Manually control graceful degradation mode. When enabled, the plugin will continue operating in a limited mode when the database is unavailable.",

		Fields: map[string]*framework.FieldSchema{
			"enabled": {
				Type:        framework.TypeBool,
				Description: "Enable or disable graceful degradation mode",
			},
		},

		Operations: map[logical.Operation]framework.OperationHandler{
			logical.ReadOperation: &framework.PathOperation{
				Callback: b.pathDegradationRead,
			},
			logical.UpdateOperation: &framework.PathOperation{
				Callback: b.pathDegradationUpdate,
			},
			logical.CreateOperation: &framework.PathOperation{
				Callback: b.pathDegradationUpdate,
			},
		},
	}
}

func (b *Backend) pathDegradationRead(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	cfg, err := getConfig(ctx, req.Storage)
	if err != nil {
		return nil, fmt.Errorf("failed to get config: %w", err)
	}

	isDegraded := b.IsDegraded()
	manuallyEnabled := b.IsGracefulDegradationManuallyEnabled()
	autoDegraded := isDegraded && !manuallyEnabled

	degradedPools := []string{}
	healthyPools := []string{}

	registry := b.getDBRegistry()
	if registry != nil {
		connectionNames := registry.ListConnections()
		for _, name := range connectionNames {
			if b.IsPoolHealthy(name) {
				healthyPools = append(healthyPools, name)
			} else {
				degradedPools = append(degradedPools, name)
			}
		}
	}

	respData := map[string]interface{}{
		"degraded":                    isDegraded,
		"manually_enabled":            manuallyEnabled,
		"automatically_triggered":     autoDegraded,
		"config_graceful_degradation": false,
		"degraded_pools":              degradedPools,
		"healthy_pools":               healthyPools,
	}

	if cfg != nil {
		respData["config_graceful_degradation"] = cfg.GracefulDegradationMode
	}

	if isDegraded && !b.DegradedSince().IsZero() {
		respData["degraded_since"] = b.DegradedSince()
		respData["degradation_duration_seconds"] = time.Since(b.DegradedSince()).Seconds()
	}

	return &logical.Response{
		Data: respData,
	}, nil
}

func (b *Backend) pathDegradationUpdate(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	enabled, ok := data.Get("enabled").(bool)
	if !ok {
		return nil, fmt.Errorf("invalid enabled value: must be a boolean")
	}

	if enabled {
		b.SetManuallyEnabledDegradation(true)
		logging.Default().Warn("graceful_degradation_manually_enabled",
			slog.Time("timestamp", time.Now()),
		)
	} else {
		b.SetGracefulDegradation(false)
		b.SetManuallyEnabledDegradation(false)
		logging.Default().Info("graceful_degradation_manually_disabled",
			slog.Time("timestamp", time.Now()),
		)
	}

	return b.pathDegradationRead(ctx, req, data)
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

	conn, err := odbc.Connect(odbc.AppendSessionTimeout(odbc.AppendQueryTimeout(cfg.ConnectionString, cfg.QueryTimeout), cfg.SessionTimeout))
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
