package teradata

import (
	"context"

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
