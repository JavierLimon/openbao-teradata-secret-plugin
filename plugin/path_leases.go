package teradata

import (
	"context"
	"fmt"
	"time"

	"github.com/JavierLimon/openbao-teradata-secret-plugin/models"
	"github.com/openbao/openbao/sdk/v2/framework"
	"github.com/openbao/openbao/sdk/v2/logical"
)

func (b *Backend) pathLeases() *framework.Path {
	return &framework.Path{
		Pattern:         "leases",
		HelpSynopsis:    "List all credential leases",
		HelpDescription: "Lists all active credential leases managed by this plugin.",

		Operations: map[logical.Operation]framework.OperationHandler{
			logical.ListOperation: &framework.PathOperation{
				Callback: b.pathLeasesList,
			},
		},
	}
}

func (b *Backend) pathLeaseInfo() *framework.Path {
	return &framework.Path{
		Pattern:         "leases/" + framework.GenericNameRegex("lease_id"),
		HelpSynopsis:    "Get lease information",
		HelpDescription: "Retrieves detailed information about a specific lease.",

		Fields: map[string]*framework.FieldSchema{
			"lease_id": {
				Type:        framework.TypeString,
				Description: "The lease ID to retrieve",
			},
		},

		Operations: map[logical.Operation]framework.OperationHandler{
			logical.ReadOperation: &framework.PathOperation{
				Callback: b.pathLeaseInfoRead,
			},
		},
	}
}

func (b *Backend) pathLeasesList(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	leases, err := listAllLeases(ctx, req.Storage)
	if err != nil {
		return nil, fmt.Errorf("failed to list leases: %w", err)
	}

	leaseList := make([]map[string]interface{}, 0, len(leases))
	now := time.Now()

	for _, lease := range leases {
		leaseInfo := map[string]interface{}{
			"lease_id":   lease.LeaseID,
			"username":   lease.Username,
			"role_name":  lease.RoleName,
			"created_at": lease.CreatedAt.Unix(),
			"expires_at": lease.ExpiresAt.Unix(),
			"expired":    now.After(lease.ExpiresAt),
		}
		if !lease.LastRenewed.IsZero() {
			leaseInfo["last_renewed"] = lease.LastRenewed.Unix()
		}
		leaseList = append(leaseList, leaseInfo)
	}

	return &logical.Response{
		Data: map[string]interface{}{
			"leases": leaseList,
			"count":  len(leaseList),
		},
	}, nil
}

func (b *Backend) pathLeaseInfoRead(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	leaseID := data.Get("lease_id").(string)

	cred, username, err := getCredentialByLeaseID(ctx, req.Storage, leaseID)
	if err != nil {
		return nil, fmt.Errorf("failed to get lease: %w", err)
	}
	if cred == nil {
		return nil, fmt.Errorf("lease %q not found", leaseID)
	}

	role, _ := getRole(ctx, req.Storage, cred.RoleName)

	respData := map[string]interface{}{
		"lease_id":   cred.LeaseID,
		"username":   cred.Username,
		"role_name":  cred.RoleName,
		"created_at": cred.CreatedAt.Unix(),
		"expires_at": cred.ExpiresAt.Unix(),
		"expired":    cred.IsExpired(),
	}

	if !cred.LastRenewed.IsZero() {
		respData["last_renewed"] = cred.LastRenewed.Unix()
	}

	if role != nil {
		respData["default_ttl"] = role.DefaultTTL
		respData["max_ttl"] = role.MaxTTL
	}

	roleCredsCount, _ := countCredentialsByRole(ctx, req.Storage, cred.RoleName)
	respData["role_credential_count"] = roleCredsCount

	usernameFromLease := username
	if usernameFromLease == "" {
		usernameFromLease = cred.Username
	}
	respData["storage_key"] = credentialPrefix + usernameFromLease

	return &logical.Response{
		Data: respData,
	}, nil
}

func (b *Backend) pathCleanupExpired() *framework.Path {
	return &framework.Path{
		Pattern:         "leases/cleanup",
		HelpSynopsis:    "Clean up expired credentials",
		HelpDescription: "Removes expired credentials from the database and storage.",

		Operations: map[logical.Operation]framework.OperationHandler{
			logical.UpdateOperation: &framework.PathOperation{
				Callback: b.pathCleanupExpiredHandler,
			},
		},
	}
}

func (b *Backend) pathCleanupExpiredHandler(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	cfg, err := getConfig(ctx, req.Storage)
	if err != nil {
		return nil, err
	}
	if cfg == nil {
		return nil, fmt.Errorf("database configuration not found")
	}

	cleaned, err := cleanupExpiredCredentials(ctx, req.Storage, cfg.ConnectionString)
	if err != nil {
		return nil, fmt.Errorf("failed to cleanup expired credentials: %w", err)
	}

	return &logical.Response{
		Data: map[string]interface{}{
			"cleaned": cleaned,
		},
	}, nil
}

func (b *Backend) pathLeaseLookup() *framework.Path {
	return &framework.Path{
		Pattern:         "lookup-lease",
		HelpSynopsis:    "Lookup lease by ID",
		HelpDescription: "Looks up a lease by its lease ID.",

		Fields: map[string]*framework.FieldSchema{
			"lease_id": {
				Type:        framework.TypeString,
				Description: "The lease ID to lookup",
			},
		},

		Operations: map[logical.Operation]framework.OperationHandler{
			logical.UpdateOperation: &framework.PathOperation{
				Callback: b.pathLeaseLookupHandler,
			},
		},
	}
}

func (b *Backend) pathLeaseLookupHandler(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	leaseID := data.Get("lease_id").(string)

	cred, username, err := getCredentialByLeaseID(ctx, req.Storage, leaseID)
	if err != nil {
		return nil, fmt.Errorf("failed to lookup lease: %w", err)
	}
	if cred == nil {
		return nil, fmt.Errorf("lease %q not found", leaseID)
	}

	role, _ := getRole(ctx, req.Storage, cred.RoleName)

	ttl := 0
	maxTTL := 0
	if role != nil {
		ttl = role.DefaultTTL
		maxTTL = role.MaxTTL
	}

	if !cred.IsExpired() && !cred.ExpiresAt.IsZero() {
		ttl = int(time.Until(cred.ExpiresAt).Seconds())
	}

	respData := map[string]interface{}{
		"lease_id":    cred.LeaseID,
		"username":    cred.Username,
		"role_name":   cred.RoleName,
		"created_at":  cred.CreatedAt,
		"expires_at":  cred.ExpiresAt,
		"expired":     cred.IsExpired(),
		"ttl":         ttl,
		"max_ttl":     maxTTL,
		"storage_key": credentialPrefix + username,
	}

	if !cred.LastRenewed.IsZero() {
		respData["last_renewed"] = cred.LastRenewed
	}

	return &logical.Response{
		Data: respData,
	}, nil
}

var _ logical.Backend = (*Backend)(nil)

func init() {
	_ = models.Credential{}
}
