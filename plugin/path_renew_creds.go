package teradata

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/JavierLimon/openbao-teradata-secret-plugin/odbc"
	"github.com/JavierLimon/openbao-teradata-secret-plugin/retry"
	"github.com/JavierLimon/openbao-teradata-secret-plugin/security"
	"github.com/openbao/openbao/sdk/v2/framework"
	"github.com/openbao/openbao/sdk/v2/logical"
)

func (b *Backend) pathRenewCreds() *framework.Path {
	return &framework.Path{
		Pattern:         "renew-cred/" + framework.GenericNameRegex("username"),
		HelpSynopsis:    "Renew database credentials",
		HelpDescription: "Rotates the password for dynamically generated database credentials.",

		Fields: map[string]*framework.FieldSchema{
			"username": {
				Type:        framework.TypeString,
				Description: "Username whose credentials to renew",
			},
		},

		Operations: map[logical.Operation]framework.OperationHandler{
			logical.UpdateOperation: &framework.PathOperation{
				Callback: b.pathRenewCredsHandler,
			},
		},
	}
}

func (b *Backend) pathRenewCredsHandler(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
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
	if role == nil {
		return nil, fmt.Errorf("role %q not found for credential", cred.RoleName)
	}

	cfg, err := getConfig(ctx, req.Storage)
	if err != nil {
		return nil, err
	}
	if cfg == nil {
		return nil, fmt.Errorf("database configuration not found")
	}

	newPassword := generatePassword()

	if err := security.ValidatePassword(newPassword); err != nil {
		return nil, fmt.Errorf("generated password validation failed: %w", err)
	}

	var conn *odbc.Connection
	err = retry.Do(ctx, nil, func() error {
		conn, err = odbc.Connect(cfg.ConnectionString)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database after retries: %w", err)
	}
	defer conn.Close()

	err = retry.Do(ctx, nil, func() error {
		return odbc.AlterUserPassword(conn.DB(), username, newPassword)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to rotate password: %w", err)
	}

	renewalStatement := role.RenewalStatement
	if role.StatementTemplate != "" {
		statement, err := getStatement(ctx, req.Storage, role.StatementTemplate)
		if err != nil {
			return nil, fmt.Errorf("failed to load statement template: %w", err)
		}
		if statement != nil && statement.RenewalStatement != "" {
			renewalStatement = statement.RenewalStatement
		}
	}

	if renewalStatement != "" {
		renewalSQL := renewalStatement
		renewalSQL = strings.ReplaceAll(renewalSQL, "{{username}}", username)
		renewalSQL = strings.ReplaceAll(renewalSQL, "{{password}}", newPassword)

		err = retry.Do(ctx, nil, func() error {
			return conn.ExecuteMultipleStatements(renewalSQL)
		})
		if err != nil {
			return nil, fmt.Errorf("failed to execute renewal statement: %w", err)
		}
	}

	ttl := time.Duration(role.DefaultTTL) * time.Second
	maxTTL := time.Duration(role.MaxTTL) * time.Second

	cred.LastRenewed = time.Now()
	cred.ExpiresAt = time.Now().Add(ttl)

	if err := storeCredential(ctx, req.Storage, username, cred); err != nil {
		return nil, fmt.Errorf("failed to update credential: %w", err)
	}
	b.cacheCredential(username, cred)

	return &logical.Response{
		Data: map[string]interface{}{
			"username":     username,
			"password":     newPassword,
			"ttl":          int(ttl.Seconds()),
			"max_ttl":      int(maxTTL.Seconds()),
			"last_renewed": cred.LastRenewed.Unix(),
			"expires_at":   cred.ExpiresAt.Unix(),
		},
	}, nil
}
