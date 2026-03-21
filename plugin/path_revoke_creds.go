package teradata

import (
	"context"
	"fmt"

	"github.com/JavierLimon/openbao-teradata-secret-plugin/odbc"
	"github.com/JavierLimon/openbao-teradata-secret-plugin/retry"
	"github.com/openbao/openbao/sdk/v2/framework"
	"github.com/openbao/openbao/sdk/v2/logical"
)

func (b *Backend) pathRevokeCreds() *framework.Path {
	return &framework.Path{
		Pattern:         "revoke-cred/" + framework.GenericNameRegex("username"),
		HelpSynopsis:    "Revoke database credentials",
		HelpDescription: "Revokes and deletes dynamically generated database credentials.",

		Fields: map[string]*framework.FieldSchema{
			"username": {
				Type:        framework.TypeString,
				Description: "Username whose credentials to revoke",
			},
		},

		Operations: map[logical.Operation]framework.OperationHandler{
			logical.DeleteOperation: &framework.PathOperation{
				Callback: b.pathRevokeCredsHandler,
			},
		},
	}
}

func (b *Backend) pathRevokeCredsHandler(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	username := data.Get("username").(string)

	if err := odbc.ValidateUsername(username); err != nil {
		return nil, fmt.Errorf("invalid username: %w", err)
	}

	cred, err := b.getCachedCredential(ctx, req.Storage, username)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve credential: %w", err)
	}
	if cred == nil {
		return nil, fmt.Errorf("credential for user %q not found", username)
	}

	role, err := getRole(ctx, req.Storage, cred.RoleName)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve role: %w", err)
	}

	cfg, err := getConfig(ctx, req.Storage)
	if err != nil {
		return nil, err
	}
	if cfg == nil {
		return nil, fmt.Errorf("database configuration not found")
	}

	var conn *odbc.Connection
	connString := odbc.AppendSessionTimeout(cfg.ConnectionString, cfg.SessionTimeout)
	connString = odbc.AppendQueryTimeout(connString, cfg.QueryTimeout)
	err = retry.Do(ctx, nil, func() error {
		conn, err = odbc.Connect(connString)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database after retries: %w", err)
	}
	defer conn.Close()

	if role != nil && role.RevocationStatement != "" {
		revocationSQL := role.RevocationStatement
		revocationSQL = replaceVariables(revocationSQL, username, "")
		err = retry.Do(ctx, nil, func() error {
			return conn.ExecuteMultipleStatements(ctx, revocationSQL)
		})
		if err != nil {
			return nil, fmt.Errorf("failed to execute revocation statement: %w", err)
		}
	}

	err = retry.Do(ctx, nil, func() error {
		return odbc.DropUser(ctx, conn.DB(), username)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to drop user: %w", err)
	}

	if err := deleteCredential(ctx, req.Storage, username); err != nil {
		return nil, fmt.Errorf("failed to delete credential: %w", err)
	}
	b.invalidateCachedCredential(username)

	return &logical.Response{
		Data: map[string]interface{}{
			"revoked": true,
		},
	}, nil
}

func replaceVariables(sql, username, password string) string {
	sql = replaceVariable(sql, "{{username}}", username)
	sql = replaceVariable(sql, "{{password}}", password)
	return sql
}

func replaceVariable(sql, placeholder, value string) string {
	if value == "" {
		return sql
	}
	result := sql
	for {
		old := result
		result = replaceAllString(result, placeholder, value)
		if old == result {
			break
		}
	}
	return result
}

func replaceAllString(s, old, new string) string {
	result := ""
	for i := 0; i < len(s); i++ {
		if i+len(old) <= len(s) && s[i:i+len(old)] == old {
			result += new
			i += len(old) - 1
		} else {
			result += string(s[i])
		}
	}
	return result
}
