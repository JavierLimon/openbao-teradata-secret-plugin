package teradata

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/JavierLimon/openbao-teradata-secret-plugin/models"
	"github.com/JavierLimon/openbao-teradata-secret-plugin/odbc"
	"github.com/JavierLimon/openbao-teradata-secret-plugin/retry"
	"github.com/JavierLimon/openbao-teradata-secret-plugin/security"
	"github.com/openbao/openbao/sdk/v2/framework"
	"github.com/openbao/openbao/sdk/v2/logical"
)

func (b *Backend) pathRenewCredsBatch() *framework.Path {
	return &framework.Path{
		Pattern:         "renew-cred/batch/" + framework.GenericNameRegex("name"),
		HelpSynopsis:    "Batch renew database credentials",
		HelpDescription: "Renews passwords for multiple dynamically generated database credentials for a role.",

		Fields: map[string]*framework.FieldSchema{
			"name": {
				Type:        framework.TypeString,
				Description: "Name of the role",
			},
			"count": {
				Type:        framework.TypeInt,
				Description: "Number of credentials to renew (default: batch_size from role, max: 100)",
			},
			"usernames": {
				Type:        framework.TypeCommaStringSlice,
				Description: "Specific usernames to renew (if not provided, renews by role up to count)",
			},
		},

		Operations: map[logical.Operation]framework.OperationHandler{
			logical.UpdateOperation: &framework.PathOperation{
				Callback: b.pathRenewCredsBatchHandler,
			},
		},
	}
}

type batchRenewalResult struct {
	Username    string    `json:"username"`
	NewPassword string    `json:"new_password"`
	TTL         int       `json:"ttl"`
	MaxTTL      int       `json:"max_ttl"`
	LastRenewed time.Time `json:"last_renewed"`
	ExpiresAt   time.Time `json:"expires_at"`
	Error       string    `json:"error,omitempty"`
}

func (b *Backend) pathRenewCredsBatchHandler(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	name := data.Get("name").(string)
	count := data.Get("count").(int)
	usernamesRaw := data.Get("usernames").([]string)

	role, err := getRole(ctx, req.Storage, name)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve role: %w", err)
	}
	if role == nil {
		return nil, fmt.Errorf("role %q not found", name)
	}

	cfg, err := getConfig(ctx, req.Storage)
	if err != nil {
		return nil, err
	}
	if cfg == nil {
		return nil, fmt.Errorf("database configuration not found")
	}

	if role.BatchSize > 0 && count <= 0 {
		count = role.BatchSize
	}
	if count <= 0 {
		count = 1
	}
	if count > 100 {
		count = 100
	}

	var usernames []string
	if len(usernamesRaw) > 0 {
		usernames = make([]string, 0, len(usernamesRaw))
		for _, u := range usernamesRaw {
			if err := odbc.ValidateUsername(u); err != nil {
				return nil, fmt.Errorf("invalid username %q: %w", u, err)
			}
			usernames = append(usernames, u)
		}
		if len(usernames) > count {
			usernames = usernames[:count]
		}
	} else {
		creds, err := b.getCredentialsByRole(ctx, req.Storage, name, count)
		if err != nil {
			return nil, fmt.Errorf("failed to get credentials for role: %w", err)
		}
		usernames = make([]string, 0, len(creds))
		for _, cred := range creds {
			usernames = append(usernames, cred.Username)
		}
	}

	if len(usernames) == 0 {
		return nil, fmt.Errorf("no credentials found to renew")
	}

	connString := odbc.AppendQueryTimeout(cfg.ConnectionString, cfg.QueryTimeout)
	conn, err := odbc.Connect(connString)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}
	defer conn.Close()

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

	ttl := time.Duration(role.DefaultTTL) * time.Second
	maxTTL := time.Duration(role.MaxTTL) * time.Second

	results := make([]batchRenewalResult, 0, len(usernames))
	successCount := 0
	failCount := 0

	for _, username := range usernames {
		result := batchRenewalResult{
			Username: username,
		}

		cred, err := b.getCachedCredential(ctx, req.Storage, username)
		if err != nil {
			result.Error = fmt.Sprintf("failed to retrieve credential: %v", err)
			results = append(results, result)
			failCount++
			continue
		}
		if cred == nil {
			result.Error = "credential not found"
			results = append(results, result)
			failCount++
			continue
		}

		newPassword := generatePassword()
		if err := security.ValidatePassword(newPassword); err != nil {
			result.Error = fmt.Sprintf("password validation failed: %v", err)
			results = append(results, result)
			failCount++
			continue
		}

		err = retry.Do(ctx, nil, func() error {
			return odbc.AlterUserPassword(ctx, conn.DB(), username, newPassword)
		})
		if err != nil {
			result.Error = fmt.Sprintf("failed to rotate password: %v", err)
			results = append(results, result)
			failCount++
			continue
		}

		if renewalStatement != "" {
			renewalSQL := renewalStatement
			renewalSQL = strings.ReplaceAll(renewalSQL, "{{username}}", username)
			renewalSQL = strings.ReplaceAll(renewalSQL, "{{password}}", newPassword)

			err = retry.Do(ctx, nil, func() error {
				return conn.ExecuteMultipleStatements(ctx, renewalSQL)
			})
			if err != nil {
				result.Error = fmt.Sprintf("failed to execute renewal statement: %v", err)
				results = append(results, result)
				failCount++
				continue
			}
		}

		cred.LastRenewed = time.Now()
		cred.ExpiresAt = time.Now().Add(ttl)

		if err := storeCredential(ctx, req.Storage, username, cred); err != nil {
			result.Error = fmt.Sprintf("failed to store credential: %v", err)
			results = append(results, result)
			failCount++
			continue
		}
		b.cacheCredential(username, cred)

		result.NewPassword = newPassword
		result.TTL = int(ttl.Seconds())
		result.MaxTTL = int(maxTTL.Seconds())
		result.LastRenewed = cred.LastRenewed
		result.ExpiresAt = cred.ExpiresAt

		results = append(results, result)
		successCount++
	}

	respData := map[string]interface{}{
		"results":       results,
		"success_count": successCount,
		"fail_count":    failCount,
		"total_count":   len(usernames),
		"ttl":           int(ttl.Seconds()),
		"max_ttl":       int(maxTTL.Seconds()),
	}

	return &logical.Response{
		Data: respData,
	}, nil
}

func (b *Backend) getCredentialsByRole(ctx context.Context, storage logical.Storage, roleName string, limit int) ([]*models.Credential, error) {
	entries, err := storage.List(ctx, credentialPrefix)
	if err != nil {
		return nil, err
	}

	creds := make([]*models.Credential, 0, limit)
	for _, entry := range entries {
		if len(creds) >= limit {
			break
		}

		username := strings.TrimPrefix(entry, credentialPrefix)
		cred, err := getCredential(ctx, storage, username)
		if err != nil {
			continue
		}
		if cred != nil && cred.RoleName == roleName {
			creds = append(creds, cred)
		}
	}

	return creds, nil
}
