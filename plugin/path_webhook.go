package teradata

import (
	"context"

	"github.com/JavierLimon/openbao-teradata-secret-plugin/webhook"
	"github.com/openbao/openbao/sdk/v2/framework"
	"github.com/openbao/openbao/sdk/v2/logical"
)

func (b *Backend) pathWebhook() *framework.Path {
	return &framework.Path{
		Pattern:         "webhook/config",
		HelpSynopsis:    "Configure webhook notifications",
		HelpDescription: "Configures webhook URL and events for external notifications.",

		Fields: map[string]*framework.FieldSchema{
			"url": {
				Type:        framework.TypeString,
				Description: "Webhook URL to send notifications to",
			},
			"enabled": {
				Type:        framework.TypeBool,
				Description: "Enable or disable webhook notifications",
				Default:     false,
			},
			"timeout_seconds": {
				Type:        framework.TypeInt,
				Description: "Webhook request timeout in seconds",
				Default:     30,
			},
			"retry_count": {
				Type:        framework.TypeInt,
				Description: "Number of times to retry failed webhook requests",
				Default:     3,
			},
			"events": {
				Type:        framework.TypeCommaStringSlice,
				Description: "List of events to send webhooks for (comma-separated)",
			},
		},

		Operations: map[logical.Operation]framework.OperationHandler{
			logical.CreateOperation: &framework.PathOperation{
				Callback: b.pathWebhookWrite,
			},
			logical.ReadOperation: &framework.PathOperation{
				Callback: b.pathWebhookRead,
			},
			logical.UpdateOperation: &framework.PathOperation{
				Callback: b.pathWebhookWrite,
			},
			logical.DeleteOperation: &framework.PathOperation{
				Callback: b.pathWebhookDelete,
			},
		},
	}
}

func (b *Backend) pathWebhookWrite(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	url := data.Get("url").(string)
	enabled := data.Get("enabled").(bool)
	timeoutSeconds := data.Get("timeout_seconds").(int)
	retryCount := data.Get("retry_count").(int)
	eventsRaw := data.Get("events").([]string)

	if timeoutSeconds <= 0 {
		timeoutSeconds = 30
	}
	if retryCount < 0 {
		retryCount = 0
	}

	if eventsRaw == nil {
		eventsRaw = []string{}
	}

	cfg := &webhook.WebhookConfig{
		URL:            url,
		Enabled:        enabled,
		TimeoutSeconds: timeoutSeconds,
		RetryCount:     retryCount,
		Events:         eventsRaw,
	}

	if err := webhook.StoreWebhookConfig(ctx, req.Storage, cfg); err != nil {
		return nil, err
	}

	return &logical.Response{
		Data: map[string]interface{}{
			"url":             url,
			"enabled":         enabled,
			"timeout_seconds": timeoutSeconds,
			"retry_count":     retryCount,
			"events":          eventsRaw,
		},
	}, nil
}

func (b *Backend) pathWebhookRead(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	cfg, err := webhook.GetWebhookConfig(ctx, req.Storage)
	if err != nil {
		return nil, err
	}

	return &logical.Response{
		Data: map[string]interface{}{
			"url":             cfg.URL,
			"enabled":         cfg.Enabled,
			"timeout_seconds": cfg.TimeoutSeconds,
			"retry_count":     cfg.RetryCount,
			"events":          cfg.Events,
		},
	}, nil
}

func (b *Backend) pathWebhookDelete(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	err := req.Storage.Delete(ctx, "webhook/config")
	if err != nil {
		return nil, err
	}

	return nil, nil
}
