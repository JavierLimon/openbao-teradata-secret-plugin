//go:build integration
// +build integration

package teradata

import (
	"context"
	"testing"
	"time"

	"github.com/openbao/openbao/sdk/v2/framework"
	"github.com/openbao/openbao/sdk/v2/logical"
)

func TestExtendLeaseHandler(t *testing.T) {
	b, storage := setupTestBackend(t)
	ctx := context.Background()

	batchStorage := newInMemoryStorage()
	b2 := NewBackend()
	cfg := &logical.BackendConfig{
		StorageView: batchStorage,
	}
	if err := b2.Setup(ctx, cfg); err != nil {
		t.Fatalf("failed to setup backend: %v", err)
	}

	configureTestDatabase(t, b, storage)
	createTestRole(t, b, storage, "extend-test", 3600, 86400)

	username, _, _ := generateTestCredential(t, b, storage, "extend-test")
	cred := getCredentialFromStorage(t, ctx, storage, username)
	leaseID := cred.LeaseID

	originalExpiresAt := cred.ExpiresAt
	time.Sleep(1 * time.Second)

	data := map[string]interface{}{
		"lease_id": leaseID,
	}

	req := &logical.Request{
		Operation: logical.UpdateOperation,
		Path:      "leases/extend/" + leaseID,
		Storage:   storage,
		Data:      data,
	}

	resp, err := b.pathExtendLeaseHandler(ctx, req, &framework.FieldData{
		Raw:    data,
		Schema: b.pathExtendLease().Fields,
	})
	if err != nil {
		t.Fatalf("failed to extend lease: %v", err)
	}
	if resp == nil {
		t.Fatal("response should not be nil")
	}
	if resp.IsError() {
		t.Fatalf("response is error: %v", resp.Error())
	}

	if resp.Data["lease_id"] != leaseID {
		t.Errorf("expected lease_id %s, got %s", leaseID, resp.Data["lease_id"])
	}
	if resp.Data["username"] != username {
		t.Errorf("expected username %s, got %s", username, resp.Data["username"])
	}

	updatedCred := getCredentialFromStorage(t, ctx, storage, username)
	if !updatedCred.ExpiresAt.After(originalExpiresAt) {
		t.Error("expiration should be extended after lease extension")
	}

	revokeTestCredential(t, b, storage, username)
}

func TestExtendLeaseWithCustomTTL(t *testing.T) {
	b, storage := setupTestBackend(t)
	ctx := context.Background()

	configureTestDatabase(t, b, storage)
	createTestRole(t, b, storage, "extend-ttl-test", 3600, 86400)

	username, _, _ := generateTestCredential(t, b, storage, "extend-ttl-test")
	cred := getCredentialFromStorage(t, ctx, storage, username)
	leaseID := cred.LeaseID

	customTTL := 7200

	data := map[string]interface{}{
		"lease_id": leaseID,
		"ttl":      customTTL,
	}

	req := &logical.Request{
		Operation: logical.UpdateOperation,
		Path:      "leases/extend/" + leaseID,
		Storage:   storage,
		Data:      data,
	}

	resp, err := b.pathExtendLeaseHandler(ctx, req, &framework.FieldData{
		Raw:    data,
		Schema: b.pathExtendLease().Fields,
	})
	if err != nil {
		t.Fatalf("failed to extend lease: %v", err)
	}
	if resp == nil {
		t.Fatal("response should not be nil")
	}

	if resp.Data["ttl"].(int) != customTTL {
		t.Errorf("expected ttl %d, got %d", customTTL, resp.Data["ttl"].(int))
	}

	revokeTestCredential(t, b, storage, username)
}

func TestExtendLeaseNotFound(t *testing.T) {
	b, storage := setupTestBackend(t)
	ctx := context.Background()

	data := map[string]interface{}{
		"lease_id": "teradata/creds/nonexistent/user123",
	}

	req := &logical.Request{
		Operation: logical.UpdateOperation,
		Path:      "leases/extend/nonexistent",
		Storage:   storage,
		Data:      data,
	}

	resp, err := b.pathExtendLeaseHandler(ctx, req, &framework.FieldData{
		Raw:    data,
		Schema: b.pathExtendLease().Fields,
	})
	if err == nil && resp == nil {
		t.Fatal("expected error for nonexistent lease")
	}
}

func TestExtendLeaseExpiredCredential(t *testing.T) {
	b, storage := setupTestBackend(t)
	ctx := context.Background()

	configureTestDatabase(t, b, storage)
	createTestRole(t, b, storage, "expired-test", 1, 5)

	username, _, _ := generateTestCredential(t, b, storage, "expired-test")
	cred := getCredentialFromStorage(t, ctx, storage, username)
	leaseID := cred.LeaseID

	cred.ExpiresAt = time.Now().Add(-1 * time.Hour)
	storage.Put(ctx, &logical.StorageEntry{
		Key:   "creds/" + username,
		Value: mustMarshalJSON(cred),
	})

	data := map[string]interface{}{
		"lease_id": leaseID,
	}

	req := &logical.Request{
		Operation: logical.UpdateOperation,
		Path:      "leases/extend/" + leaseID,
		Storage:   storage,
		Data:      data,
	}

	resp, err := b.pathExtendLeaseHandler(ctx, req, &framework.FieldData{
		Raw:    data,
		Schema: b.pathExtendLease().Fields,
	})
	if err == nil && resp == nil {
		t.Fatal("expected error for expired lease")
	}
}

func TestExtendLeaseMaxTTLExceeded(t *testing.T) {
	b, storage := setupTestBackend(t)
	ctx := context.Background()

	configureTestDatabase(t, b, storage)
	createTestRole(t, b, storage, "max-ttl-test", 3600, 5)

	username, _, _ := generateTestCredential(t, b, storage, "max-ttl-test")
	cred := getCredentialFromStorage(t, ctx, storage, username)
	leaseID := cred.LeaseID

	cred.CreatedAt = time.Now().Add(-10 * time.Second)
	storage.Put(ctx, &logical.StorageEntry{
		Key:   "creds/" + username,
		Value: mustMarshalJSON(cred),
	})

	data := map[string]interface{}{
		"lease_id": leaseID,
	}

	req := &logical.Request{
		Operation: logical.UpdateOperation,
		Path:      "leases/extend/" + leaseID,
		Storage:   storage,
		Data:      data,
	}

	resp, err := b.pathExtendLeaseHandler(ctx, req, &framework.FieldData{
		Raw:    data,
		Schema: b.pathExtendLease().Fields,
	})
	if err == nil && resp == nil {
		t.Fatal("expected error when max TTL exceeded")
	}
}

func mustMarshalJSON(v interface{}) []byte {
	b, err := logical.StorageEntryJSON("", v)
	if err != nil {
		panic(err)
	}
	return b.Value
}
