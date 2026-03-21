package teradata

import (
	"context"

	"github.com/openbao/openbao/sdk/v2/framework"
	"github.com/openbao/openbao/sdk/v2/logical"
)

func (b *Backend) pathRateLimitConfig() *framework.Path {
	return &framework.Path{
		Pattern:         "rate-limit/config",
		HelpSynopsis:    "Configure rate limiting",
		HelpDescription: "Configures rate limiting for API requests to prevent abuse.",

		Fields: map[string]*framework.FieldSchema{
			"enabled": {
				Type:        framework.TypeBool,
				Description: "Enable or disable rate limiting",
			},
			"requests_per_second": {
				Type:        framework.TypeFloat,
				Description: "Maximum requests per second per IP",
			},
			"burst_size": {
				Type:        framework.TypeInt,
				Description: "Maximum burst size for rate limiting",
			},
		},

		Operations: map[logical.Operation]framework.OperationHandler{
			logical.ReadOperation: &framework.PathOperation{
				Callback: b.pathRateLimitConfigRead,
			},
			logical.UpdateOperation: &framework.PathOperation{
				Callback: b.pathRateLimitConfigWrite,
			},
		},
	}
}

func (b *Backend) pathRateLimitStatus() *framework.Path {
	return &framework.Path{
		Pattern:         "rate-limit/status",
		HelpSynopsis:    "Check rate limiting status",
		HelpDescription: "Returns the current rate limiting status and statistics.",

		Operations: map[logical.Operation]framework.OperationHandler{
			logical.ReadOperation: &framework.PathOperation{
				Callback: b.pathRateLimitStatusRead,
			},
		},
	}
}

func (b *Backend) pathRateLimitConfigRead(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	cfg := b.GetRateLimitConfig()
	enabled := b.IsRateLimiterEnabled()

	return &logical.Response{
		Data: map[string]interface{}{
			"enabled":                  enabled,
			"requests_per_second":      cfg.RequestsPerSecond,
			"burst_size":               cfg.BurstSize,
			"cleanup_interval_seconds": int(cfg.CleanupInterval.Seconds()),
		},
	}, nil
}

func (b *Backend) pathRateLimitConfigWrite(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	if enabled, ok := data.Raw["enabled"].(bool); ok {
		b.SetRateLimiterEnabled(enabled)
	}

	return b.pathRateLimitConfigRead(ctx, req, data)
}

func (b *Backend) pathRateLimitStatusRead(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	cfg := b.GetRateLimitConfig()
	enabled := b.IsRateLimiterEnabled()

	return &logical.Response{
		Data: map[string]interface{}{
			"enabled":                  enabled,
			"requests_per_second":      cfg.RequestsPerSecond,
			"burst_size":               cfg.BurstSize,
			"cleanup_interval_seconds": int(cfg.CleanupInterval.Seconds()),
		},
	}, nil
}
