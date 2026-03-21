package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/JavierLimon/openbao-teradata-secret-plugin/retry"
	"github.com/openbao/openbao/sdk/v2/logical"
)

type WebhookEventType string

const (
	EventTypeCredentialCreated WebhookEventType = "credential_created"
	EventTypeCredentialRevoked WebhookEventType = "credential_revoked"
	EventTypeCredentialRotated WebhookEventType = "credential_rotated"
	EventTypeRoleCreated       WebhookEventType = "role_created"
	EventTypeRoleUpdated       WebhookEventType = "role_updated"
	EventTypeRoleDeleted       WebhookEventType = "role_deleted"
	EventTypeRootRotated       WebhookEventType = "root_rotated"
)

type WebhookConfig struct {
	URL            string   `json:"url"`
	Enabled        bool     `json:"enabled"`
	TimeoutSeconds int      `json:"timeout_seconds"`
	RetryCount     int      `json:"retry_count"`
	Events         []string `json:"events"`
}

type WebhookPayload struct {
	EventType string                 `json:"event_type"`
	Timestamp time.Time              `json:"timestamp"`
	Data      map[string]interface{} `json:"data"`
}

const (
	defaultTimeout = 30
	defaultRetries = 3
)

func NewWebhookConfig() *WebhookConfig {
	return &WebhookConfig{
		Enabled:        false,
		TimeoutSeconds: defaultTimeout,
		RetryCount:     defaultRetries,
		Events: []string{
			string(EventTypeCredentialCreated),
			string(EventTypeCredentialRevoked),
			string(EventTypeCredentialRotated),
			string(EventTypeRoleCreated),
			string(EventTypeRoleUpdated),
			string(EventTypeRoleDeleted),
			string(EventTypeRootRotated),
		},
	}
}

func (c *WebhookConfig) IsEventEnabled(eventType string) bool {
	if !c.Enabled {
		return false
	}
	for _, e := range c.Events {
		if e == eventType {
			return true
		}
	}
	return false
}

func SendWebhook(ctx context.Context, config *WebhookConfig, eventType WebhookEventType, data map[string]interface{}) error {
	if config == nil || !config.IsEventEnabled(string(eventType)) {
		return nil
	}

	payload := WebhookPayload{
		EventType: string(eventType),
		Timestamp: time.Now().UTC(),
		Data:      data,
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal webhook payload: %w", err)
	}

	timeout := time.Duration(config.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = defaultTimeout * time.Second
	}

	client := &http.Client{
		Timeout: timeout,
	}

	retryCount := config.RetryCount
	if retryCount <= 0 {
		retryCount = defaultRetries
	}

	retryCfg := &retry.Config{
		MaxAttempts:     retryCount,
		InitialInterval: 100 * time.Millisecond,
		MaxInterval:     5 * time.Second,
		Multiplier:      2.0,
	}

	var lastErr error
	err = retry.Do(ctx, retryCfg, func() error {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, config.URL, bytes.NewBuffer(jsonPayload))
		if err != nil {
			lastErr = fmt.Errorf("failed to create webhook request: %w", err)
			return err
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", "openbao-teradata-secret-plugin/1.0")
		req.Header.Set("X-Webhook-Event", string(eventType))

		resp, err := client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("webhook request failed: %w", err)
			return retry.NewTransientError(err)
		}

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			resp.Body.Close()
			return nil
		}

		lastErr = fmt.Errorf("webhook returned status %d", resp.StatusCode)
		resp.Body.Close()
		if resp.StatusCode >= 500 {
			return retry.NewTransientError(lastErr)
		}
		return lastErr
	})

	if err != nil {
		return lastErr
	}
	return nil
}

func SendCredentialCreatedWebhook(ctx context.Context, storage logical.Storage, username, roleName, leaseID string, metadata map[string]interface{}) error {
	config, err := GetWebhookConfig(ctx, storage)
	if err != nil {
		return err
	}

	data := map[string]interface{}{
		"username":  username,
		"role_name": roleName,
		"lease_id":  leaseID,
		"timestamp": time.Now().UTC(),
	}
	if metadata != nil {
		data["metadata"] = metadata
	}

	return SendWebhook(ctx, config, EventTypeCredentialCreated, data)
}

func SendCredentialRevokedWebhook(ctx context.Context, storage logical.Storage, username, roleName string, metadata map[string]interface{}) error {
	config, err := GetWebhookConfig(ctx, storage)
	if err != nil {
		return err
	}

	data := map[string]interface{}{
		"username":  username,
		"role_name": roleName,
		"timestamp": time.Now().UTC(),
	}
	if metadata != nil {
		data["metadata"] = metadata
	}

	return SendWebhook(ctx, config, EventTypeCredentialRevoked, data)
}

func SendRoleCreatedWebhook(ctx context.Context, storage logical.Storage, roleName, dbUser, statement string) error {
	config, err := GetWebhookConfig(ctx, storage)
	if err != nil {
		return err
	}

	data := map[string]interface{}{
		"role_name": roleName,
		"db_user":   dbUser,
		"statement": statement,
		"timestamp": time.Now().UTC(),
	}

	return SendWebhook(ctx, config, EventTypeRoleCreated, data)
}

func SendRoleUpdatedWebhook(ctx context.Context, storage logical.Storage, roleName, dbUser, statement string) error {
	config, err := GetWebhookConfig(ctx, storage)
	if err != nil {
		return err
	}

	data := map[string]interface{}{
		"role_name": roleName,
		"db_user":   dbUser,
		"statement": statement,
		"timestamp": time.Now().UTC(),
	}

	return SendWebhook(ctx, config, EventTypeRoleUpdated, data)
}

func SendRoleDeletedWebhook(ctx context.Context, storage logical.Storage, roleName string) error {
	config, err := GetWebhookConfig(ctx, storage)
	if err != nil {
		return err
	}

	data := map[string]interface{}{
		"role_name": roleName,
		"timestamp": time.Now().UTC(),
	}

	return SendWebhook(ctx, config, EventTypeRoleDeleted, data)
}

func SendRootRotatedWebhook(ctx context.Context, storage logical.Storage, success bool, errMsg string) error {
	config, err := GetWebhookConfig(ctx, storage)
	if err != nil {
		return err
	}

	data := map[string]interface{}{
		"success":   success,
		"error":     errMsg,
		"timestamp": time.Now().UTC(),
	}

	return SendWebhook(ctx, config, EventTypeRootRotated, data)
}

func GetWebhookConfig(ctx context.Context, storage logical.Storage) (*WebhookConfig, error) {
	entry, err := storage.Get(ctx, "webhook/config")
	if err != nil {
		return nil, err
	}

	if entry == nil {
		return NewWebhookConfig(), nil
	}

	var config WebhookConfig
	if err := entry.DecodeJSON(&config); err != nil {
		return nil, err
	}

	return &config, nil
}

func StoreWebhookConfig(ctx context.Context, storage logical.Storage, config *WebhookConfig) error {
	if config == nil {
		config = NewWebhookConfig()
	}

	entry, err := logical.StorageEntryJSON("webhook/config", config)
	if err != nil {
		return err
	}

	return storage.Put(ctx, entry)
}
