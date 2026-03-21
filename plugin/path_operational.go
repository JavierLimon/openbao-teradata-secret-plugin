package teradata

import (
	"context"
	"fmt"

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

	return &logical.Response{
		Data: map[string]interface{}{
			"status":      "healthy",
			"initialized": true,
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

	conn, err := odbc.Connect(cfg.ConnectionString)
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
