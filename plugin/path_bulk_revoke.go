package teradata

import (
	"context"
	"fmt"
	"strings"

	"github.com/JavierLimon/openbao-teradata-secret-plugin/audit"
	"github.com/JavierLimon/openbao-teradata-secret-plugin/models"
	"github.com/JavierLimon/openbao-teradata-secret-plugin/odbc"
	"github.com/JavierLimon/openbao-teradata-secret-plugin/retry"
	"github.com/JavierLimon/openbao-teradata-secret-plugin/webhook"
	"github.com/openbao/openbao/sdk/v2/framework"
	"github.com/openbao/openbao/sdk/v2/logical"
)

func (b *Backend) pathBulkRevokeCreds() *framework.Path {
	return &framework.Path{
		Pattern:         "bulk-revoke-creds",
		HelpSynopsis:    "Bulk revoke database credentials",
		HelpDescription: "Revokes and deletes multiple dynamically generated database credentials at once.",

		Fields: map[string]*framework.FieldSchema{
			"usernames": {
				Type:        framework.TypeStringSlice,
				Description: "List of usernames whose credentials to revoke",
			},
		},

		Operations: map[logical.Operation]framework.OperationHandler{
			logical.UpdateOperation: &framework.PathOperation{
				Callback: b.pathBulkRevokeCredsHandler,
			},
		},
	}
}

type bulkRevokeResult struct {
	Username string `json:"username"`
	Success  bool   `json:"success"`
	Error    string `json:"error,omitempty"`
}

type bulkRevokeResponse struct {
	Revoked int                `json:"revoked"`
	Failed  int                `json:"failed"`
	Results []bulkRevokeResult `json:"results"`
}

func (b *Backend) pathBulkRevokeCredsHandler(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	usernamesRaw, ok := data.GetOk("usernames")
	if !ok {
		return nil, fmt.Errorf("usernames parameter is required")
	}

	usernames, ok := usernamesRaw.([]string)
	if !ok || len(usernames) == 0 {
		return nil, fmt.Errorf("usernames must be a non-empty list")
	}

	cfg, err := getConfig(ctx, req.Storage)
	if err != nil {
		return nil, err
	}
	if cfg == nil {
		return nil, fmt.Errorf("database configuration not found")
	}

	results := make([]bulkRevokeResult, 0, len(usernames))
	revoked := 0
	failed := 0

	connString := odbc.AppendQueryTimeout(cfg.ConnectionString, cfg.QueryTimeout)

	for _, username := range usernames {
		result := bulkRevokeResult{Username: username}

		if err := odbc.ValidateUsername(username); err != nil {
			result.Success = false
			result.Error = fmt.Sprintf("invalid username: %v", err)
			results = append(results, result)
			failed++
			continue
		}

		cred, err := b.getCachedCredential(ctx, req.Storage, username)
		if err != nil {
			result.Success = false
			result.Error = fmt.Sprintf("failed to retrieve credential: %v", err)
			results = append(results, result)
			failed++
			continue
		}
		if cred == nil {
			result.Success = false
			result.Error = "credential not found"
			results = append(results, result)
			failed++
			continue
		}

		role, err := getRole(ctx, req.Storage, cred.RoleName)
		if err != nil {
			result.Success = false
			result.Error = fmt.Sprintf("failed to retrieve role: %v", err)
			results = append(results, result)
			failed++
			continue
		}

		var conn *odbc.Connection
		err = retry.Do(ctx, nil, func() error {
			conn, err = odbc.Connect(connString)
			return err
		})
		if err != nil {
			result.Success = false
			result.Error = fmt.Sprintf("failed to connect to database: %v", err)
			results = append(results, result)
			failed++
			continue
		}

		if role != nil && role.RevocationStatement != "" {
			revocationSQL := replaceVariables(role.RevocationStatement, username, "")
			err = retry.Do(ctx, nil, func() error {
				return conn.ExecuteMultipleStatements(ctx, revocationSQL)
			})
			if err != nil {
				conn.Close()
				result.Success = false
				result.Error = fmt.Sprintf("failed to execute revocation statement: %v", err)
				results = append(results, result)
				failed++
				continue
			}
		}

		err = retry.Do(ctx, nil, func() error {
			return odbc.DropUser(ctx, conn.DB(), username)
		})
		conn.Close()

		if err != nil {
			_ = audit.LogCredentialRevocation(ctx, req.Storage, username, cred.RoleName, map[string]interface{}{"error": err.Error()})
			_ = webhook.SendCredentialRevokedWebhook(ctx, req.Storage, username, cred.RoleName, map[string]interface{}{"error": err.Error()})
			result.Success = false
			result.Error = fmt.Sprintf("failed to drop user: %v", err)
			results = append(results, result)
			failed++
			continue
		}

		if err := deleteCredential(ctx, req.Storage, username); err != nil {
			result.Success = false
			result.Error = fmt.Sprintf("failed to delete credential from storage: %v", err)
			results = append(results, result)
			failed++
			continue
		}

		b.invalidateCachedCredential(username)
		_ = audit.LogCredentialRevocation(ctx, req.Storage, username, cred.RoleName, nil)
		_ = webhook.SendCredentialRevokedWebhook(ctx, req.Storage, username, cred.RoleName, nil)

		result.Success = true
		results = append(results, result)
		revoked++
	}

	return &logical.Response{
		Data: map[string]interface{}{
			"revoked": revoked,
			"failed":  failed,
			"results": results,
		},
	}, nil
}

func (b *Backend) pathBulkRevokeByRole() *framework.Path {
	return &framework.Path{
		Pattern:         "bulk-revoke-creds/role/" + framework.GenericNameRegex("role"),
		HelpSynopsis:    "Bulk revoke credentials by role",
		HelpDescription: "Revokes and deletes all dynamically generated database credentials for a specific role.",

		Fields: map[string]*framework.FieldSchema{
			"role": {
				Type:        framework.TypeString,
				Description: "Role name whose credentials to revoke",
			},
		},

		Operations: map[logical.Operation]framework.OperationHandler{
			logical.DeleteOperation: &framework.PathOperation{
				Callback: b.pathBulkRevokeByRoleHandler,
			},
		},
	}
}

func (b *Backend) pathBulkRevokeByRoleHandler(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	roleName := data.Get("role").(string)

	if roleName == "" {
		return nil, fmt.Errorf("role parameter is required")
	}

	cfg, err := getConfig(ctx, req.Storage)
	if err != nil {
		return nil, err
	}
	if cfg == nil {
		return nil, fmt.Errorf("database configuration not found")
	}

	role, err := getRole(ctx, req.Storage, roleName)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve role: %w", err)
	}
	if role == nil {
		return nil, fmt.Errorf("role %q not found", roleName)
	}

	entries, err := req.Storage.List(ctx, credentialPrefix)
	if err != nil {
		return nil, fmt.Errorf("failed to list credentials: %w", err)
	}

	usernames := make([]string, 0)
	for _, entry := range entries {
		username := strings.TrimPrefix(entry, credentialPrefix)
		cred, err := getCredential(ctx, req.Storage, username)
		if err != nil {
			continue
		}
		if cred != nil && cred.RoleName == roleName {
			usernames = append(usernames, username)
		}
	}

	if len(usernames) == 0 {
		return &logical.Response{
			Data: map[string]interface{}{
				"revoked": 0,
				"failed":  0,
				"results": []bulkRevokeResult{},
			},
		}, nil
	}

	connString := odbc.AppendQueryTimeout(cfg.ConnectionString, cfg.QueryTimeout)

	results := make([]bulkRevokeResult, 0, len(usernames))
	revoked := 0
	failed := 0

	for _, username := range usernames {
		result := bulkRevokeResult{Username: username}

		var conn *odbc.Connection
		err := retry.Do(ctx, nil, func() error {
			conn, err = odbc.Connect(connString)
			return err
		})
		if err != nil {
			result.Success = false
			result.Error = fmt.Sprintf("failed to connect to database: %v", err)
			results = append(results, result)
			failed++
			continue
		}

		if role.RevocationStatement != "" {
			revocationSQL := replaceVariables(role.RevocationStatement, username, "")
			err = retry.Do(ctx, nil, func() error {
				return conn.ExecuteMultipleStatements(ctx, revocationSQL)
			})
			if err != nil {
				conn.Close()
				result.Success = false
				result.Error = fmt.Sprintf("failed to execute revocation statement: %v", err)
				results = append(results, result)
				failed++
				continue
			}
		}

		err = retry.Do(ctx, nil, func() error {
			return odbc.DropUser(ctx, conn.DB(), username)
		})
		conn.Close()

		if err != nil {
			_ = audit.LogCredentialRevocation(ctx, req.Storage, username, roleName, map[string]interface{}{"error": err.Error()})
			_ = webhook.SendCredentialRevokedWebhook(ctx, req.Storage, username, roleName, map[string]interface{}{"error": err.Error()})
			result.Success = false
			result.Error = fmt.Sprintf("failed to drop user: %v", err)
			results = append(results, result)
			failed++
			continue
		}

		if err := deleteCredential(ctx, req.Storage, username); err != nil {
			result.Success = false
			result.Error = fmt.Sprintf("failed to delete credential from storage: %v", err)
			results = append(results, result)
			failed++
			continue
		}

		b.invalidateCachedCredential(username)
		_ = audit.LogCredentialRevocation(ctx, req.Storage, username, roleName, nil)
		_ = webhook.SendCredentialRevokedWebhook(ctx, req.Storage, username, roleName, nil)

		result.Success = true
		results = append(results, result)
		revoked++
	}

	return &logical.Response{
		Data: map[string]interface{}{
			"revoked": revoked,
			"failed":  failed,
			"results": results,
		},
	}, nil
}

func (b *Backend) pathRevokeAllCreds() *framework.Path {
	return &framework.Path{
		Pattern:         "bulk-revoke-creds/all",
		HelpSynopsis:    "Revoke all credentials",
		HelpDescription: "Revokes and deletes all dynamically generated database credentials.",

		Operations: map[logical.Operation]framework.OperationHandler{
			logical.DeleteOperation: &framework.PathOperation{
				Callback: b.pathRevokeAllCredsHandler,
			},
		},
	}
}

func (b *Backend) pathRevokeAllCredsHandler(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	cfg, err := getConfig(ctx, req.Storage)
	if err != nil {
		return nil, err
	}
	if cfg == nil {
		return nil, fmt.Errorf("database configuration not found")
	}

	entries, err := req.Storage.List(ctx, credentialPrefix)
	if err != nil {
		return nil, fmt.Errorf("failed to list credentials: %w", err)
	}

	if len(entries) == 0 {
		return &logical.Response{
			Data: map[string]interface{}{
				"revoked": 0,
				"failed":  0,
				"results": []bulkRevokeResult{},
			},
		}, nil
	}

	creds := make([]*models.Credential, 0, len(entries))
	for _, entry := range entries {
		username := strings.TrimPrefix(entry, credentialPrefix)
		cred, err := getCredential(ctx, req.Storage, username)
		if err != nil {
			continue
		}
		if cred != nil {
			creds = append(creds, cred)
		}
	}

	roleCache := make(map[string]*models.Role)
	getRoleCached := func(ctx context.Context, storage logical.Storage, roleName string) (*models.Role, error) {
		if role, ok := roleCache[roleName]; ok {
			return role, nil
		}
		role, err := getRole(ctx, storage, roleName)
		if err != nil {
			return nil, err
		}
		roleCache[roleName] = role
		return role, nil
	}

	connString := odbc.AppendQueryTimeout(cfg.ConnectionString, cfg.QueryTimeout)

	results := make([]bulkRevokeResult, 0, len(creds))
	revoked := 0
	failed := 0

	for _, cred := range creds {
		result := bulkRevokeResult{Username: cred.Username}

		role, err := getRoleCached(ctx, req.Storage, cred.RoleName)
		if err != nil {
			result.Success = false
			result.Error = fmt.Sprintf("failed to retrieve role: %v", err)
			results = append(results, result)
			failed++
			continue
		}

		var conn *odbc.Connection
		err = retry.Do(ctx, nil, func() error {
			conn, err = odbc.Connect(connString)
			return err
		})
		if err != nil {
			result.Success = false
			result.Error = fmt.Sprintf("failed to connect to database: %v", err)
			results = append(results, result)
			failed++
			continue
		}

		if role != nil && role.RevocationStatement != "" {
			revocationSQL := replaceVariables(role.RevocationStatement, cred.Username, "")
			err = retry.Do(ctx, nil, func() error {
				return conn.ExecuteMultipleStatements(ctx, revocationSQL)
			})
			if err != nil {
				conn.Close()
				result.Success = false
				result.Error = fmt.Sprintf("failed to execute revocation statement: %v", err)
				results = append(results, result)
				failed++
				continue
			}
		}

		err = retry.Do(ctx, nil, func() error {
			return odbc.DropUser(ctx, conn.DB(), cred.Username)
		})
		conn.Close()

		if err != nil {
			_ = audit.LogCredentialRevocation(ctx, req.Storage, cred.Username, cred.RoleName, map[string]interface{}{"error": err.Error()})
			_ = webhook.SendCredentialRevokedWebhook(ctx, req.Storage, cred.Username, cred.RoleName, map[string]interface{}{"error": err.Error()})
			result.Success = false
			result.Error = fmt.Sprintf("failed to drop user: %v", err)
			results = append(results, result)
			failed++
			continue
		}

		if err := deleteCredential(ctx, req.Storage, cred.Username); err != nil {
			result.Success = false
			result.Error = fmt.Sprintf("failed to delete credential from storage: %v", err)
			results = append(results, result)
			failed++
			continue
		}

		b.invalidateCachedCredential(cred.Username)
		_ = audit.LogCredentialRevocation(ctx, req.Storage, cred.Username, cred.RoleName, nil)
		_ = webhook.SendCredentialRevokedWebhook(ctx, req.Storage, cred.Username, cred.RoleName, nil)

		result.Success = true
		results = append(results, result)
		revoked++
	}

	return &logical.Response{
		Data: map[string]interface{}{
			"revoked": revoked,
			"failed":  failed,
			"results": results,
		},
	}, nil
}
