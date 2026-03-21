package teradata

import (
	"context"
	"fmt"
	"time"

	"github.com/openbao/openbao/sdk/v2/framework"
	"github.com/openbao/openbao/sdk/v2/logical"
)

func (b *Backend) pathExtendLease() *framework.Path {
	return &framework.Path{
		Pattern:         "leases/extend/" + framework.GenericNameRegex("lease_id"),
		HelpSynopsis:    "Extend credential lease",
		HelpDescription: "Extends the TTL of an existing credential lease without rotating the password.",

		Fields: map[string]*framework.FieldSchema{
			"lease_id": {
				Type:        framework.TypeString,
				Description: "The lease ID to extend",
			},
			"ttl": {
				Type:        framework.TypeInt,
				Description: "TTL in seconds to extend (optional, uses role default if not specified)",
			},
		},

		Operations: map[logical.Operation]framework.OperationHandler{
			logical.UpdateOperation: &framework.PathOperation{
				Callback: b.pathExtendLeaseHandler,
			},
		},
	}
}

func (b *Backend) pathExtendLeaseHandler(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	leaseID := data.Get("lease_id").(string)
	requestedTTL := data.Get("ttl").(int)

	if leaseID == "" {
		return nil, fmt.Errorf("lease_id is required")
	}

	cred, username, err := getCredentialByLeaseID(ctx, req.Storage, leaseID)
	if err != nil {
		return nil, fmt.Errorf("failed to get credential: %w", err)
	}
	if cred == nil {
		return nil, fmt.Errorf("lease %q not found", leaseID)
	}

	role, err := getRole(ctx, req.Storage, cred.RoleName)
	if err != nil {
		return nil, fmt.Errorf("failed to get role: %w", err)
	}
	if role == nil {
		return nil, fmt.Errorf("role %q not found for lease", cred.RoleName)
	}

	if cred.IsExpired() {
		return nil, fmt.Errorf("lease %q has expired and cannot be extended", leaseID)
	}

	defaultTTL := time.Duration(role.DefaultTTL) * time.Second
	maxTTL := time.Duration(role.MaxTTL) * time.Second

	if maxTTL > 0 {
		totalLeaseTime := time.Since(cred.CreatedAt)
		if totalLeaseTime >= maxTTL {
			return nil, fmt.Errorf("lease %q has reached maximum TTL of %v and cannot be extended further", leaseID, maxTTL)
		}
	}

	var ttl time.Duration
	if requestedTTL > 0 {
		ttl = time.Duration(requestedTTL) * time.Second
	} else {
		ttl = defaultTTL
	}

	if maxTTL > 0 {
		totalLeaseTime := time.Since(cred.CreatedAt)
		remainingMaxTTL := maxTTL - totalLeaseTime
		if ttl > remainingMaxTTL {
			ttl = remainingMaxTTL
		}
		if ttl <= 0 {
			return nil, fmt.Errorf("lease %q has reached maximum TTL and cannot be extended", leaseID)
		}
	}

	cred.LastRenewed = time.Now()
	cred.ExpiresAt = time.Now().Add(ttl)

	if err := storeCredential(ctx, req.Storage, username, cred); err != nil {
		return nil, fmt.Errorf("failed to update credential: %w", err)
	}
	b.cacheCredential(username, cred)

	return &logical.Response{
		Data: map[string]interface{}{
			"lease_id":     cred.LeaseID,
			"username":     cred.Username,
			"role_name":    cred.RoleName,
			"ttl":          int(ttl.Seconds()),
			"max_ttl":      int(maxTTL.Seconds()),
			"expires_at":   cred.ExpiresAt.Unix(),
			"last_renewed": cred.LastRenewed.Unix(),
		},
	}, nil
}
