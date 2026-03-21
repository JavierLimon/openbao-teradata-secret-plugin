//go:build integration
// +build integration

package teradata

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/JavierLimon/openbao-teradata-secret-plugin/models"
	"github.com/openbao/openbao/sdk/v2/framework"
	"github.com/openbao/openbao/sdk/v2/logical"
)

const (
	testRoleName = "test-role"
)

var (
	teradataHost     = getEnv("TERADATA_HOST", "testing-rhjbbw139fee5yg7.env.clearscape.teradata.com")
	teradataUser     = getEnv("TERADATA_USER", "demo_user")
	teradataPassword = getEnv("TERADATA_PASSWORD", "latve1ja")
	teradataDSN      = getEnv("TERADATA_DSN", "Teradata")
)

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getTestConnectionString() string {
	return fmt.Sprintf("DSN=%s;UID=%s;PWD=%s;", teradataDSN, teradataUser, teradataPassword)
}

type inMemoryStorage struct {
	mu   sync.RWMutex
	data map[string][]byte
}

func newInMemoryStorage() *inMemoryStorage {
	return &inMemoryStorage{
		data: make(map[string][]byte),
	}
}

func (s *inMemoryStorage) Put(ctx context.Context, entry *logical.StorageEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[entry.Key] = entry.Value
	return nil
}

func (s *inMemoryStorage) Get(ctx context.Context, key string) (*logical.StorageEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if val, ok := s.data[key]; ok {
		return &logical.StorageEntry{
			Key:   key,
			Value: val,
		}, nil
	}
	return nil, nil
}

func (s *inMemoryStorage) Delete(ctx context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, key)
	return nil
}

func (s *inMemoryStorage) List(ctx context.Context, prefix string) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var keys []string
	for k := range s.data {
		if strings.HasPrefix(k, prefix) {
			keys = append(keys, k)
		}
	}
	return keys, nil
}

func (s *inMemoryStorage) ListPage(ctx context.Context, prefix string, after string, limit int) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var keys []string
	for k := range s.data {
		if strings.HasPrefix(k, prefix) {
			keys = append(keys, k)
		}
	}
	return keys, nil
}

func (s *inMemoryStorage) Keys(ctx context.Context, prefix string) ([]string, error) {
	return s.List(ctx, prefix)
}

func setupTestBackend(t *testing.T) (*Backend, logical.Storage) {
	t.Helper()

	storage := newInMemoryStorage()

	b := NewBackend()
	ctx := context.Background()

	cfg := &logical.BackendConfig{
		StorageView: storage,
	}

	if err := b.Setup(ctx, cfg); err != nil {
		t.Fatalf("failed to setup backend: %v", err)
	}

	return b, storage
}

func configureTestDatabase(t *testing.T, b *Backend, storage logical.Storage) {
	t.Helper()

	ctx := context.Background()

	data := map[string]interface{}{
		"connection_string":    getTestConnectionString(),
		"max_open_connections": 5,
		"max_idle_connections": 2,
		"connection_timeout":   30,
	}

	req := &logical.Request{
		Operation: logical.CreateOperation,
		Path:      "config",
		Storage:   storage,
		Data:      data,
	}

	resp, err := b.pathConfigWrite(ctx, req, &framework.FieldData{
		Raw:    data,
		Schema: b.pathConfig().Fields,
	})
	if err != nil || (resp != nil && resp.IsError()) {
		t.Fatalf("failed to configure database: %v (resp: %v)", err, resp)
	}
}

func createTestRole(t *testing.T, b *Backend, storage logical.Storage, name string, defaultTTL, maxTTL int) {
	t.Helper()

	ctx := context.Background()

	data := map[string]interface{}{
		"name":                 name,
		"db_user":              "{{username}}",
		"default_ttl":          defaultTTL,
		"max_ttl":              maxTTL,
		"default_database":     name,
		"creation_statement":   "GRANT SELECT ON DBC TO {{username}};",
		"revocation_statement": "REVOKE SELECT ON DBC FROM {{username}};",
	}

	req := &logical.Request{
		Operation: logical.CreateOperation,
		Path:      "roles/" + name,
		Storage:   storage,
		Data:      data,
	}

	resp, err := b.pathRoleCreate(ctx, req, &framework.FieldData{
		Raw:    data,
		Schema: b.pathRoles().Fields,
	})
	if err != nil || (resp != nil && resp.IsError()) {
		t.Fatalf("failed to create role: %v (resp: %v)", err, resp)
	}
}

func generateTestCredential(t *testing.T, b *Backend, storage logical.Storage, roleName string) (string, string, int) {
	t.Helper()

	ctx := context.Background()

	data := map[string]interface{}{
		"name": roleName,
	}

	req := &logical.Request{
		Operation: logical.ReadOperation,
		Path:      "creds/" + roleName,
		Storage:   storage,
		Data:      data,
	}

	resp, err := b.pathCredsRead(ctx, req, &framework.FieldData{
		Raw:    data,
		Schema: b.pathCreds().Fields,
	})
	if err != nil || (resp != nil && resp.IsError()) {
		t.Fatalf("failed to generate credential: %v (resp: %v)", err, resp)
	}

	username := resp.Data["username"].(string)
	password := resp.Data["password"].(string)
	ttl := resp.Data["ttl"].(int)

	return username, password, ttl
}

func renewTestCredential(t *testing.T, b *Backend, storage logical.Storage, username string) (string, int) {
	t.Helper()

	ctx := context.Background()

	data := map[string]interface{}{
		"username": username,
	}

	req := &logical.Request{
		Operation: logical.UpdateOperation,
		Path:      "renew-cred/" + username,
		Storage:   storage,
		Data:      data,
	}

	resp, err := b.pathRenewCredsHandler(ctx, req, &framework.FieldData{
		Raw:    data,
		Schema: b.pathRenewCreds().Fields,
	})
	if err != nil || (resp != nil && resp.IsError()) {
		t.Fatalf("failed to renew credential: %v (resp: %v)", err, resp)
	}

	newPassword := resp.Data["password"].(string)
	ttl := resp.Data["ttl"].(int)

	return newPassword, ttl
}

func revokeTestCredential(t *testing.T, b *Backend, storage logical.Storage, username string) {
	t.Helper()

	ctx := context.Background()

	data := map[string]interface{}{
		"username": username,
	}

	req := &logical.Request{
		Operation: logical.DeleteOperation,
		Path:      "revoke-cred/" + username,
		Storage:   storage,
		Data:      data,
	}

	resp, err := b.pathRevokeCredsHandler(ctx, req, &framework.FieldData{
		Raw:    data,
		Schema: b.pathRevokeCreds().Fields,
	})
	if err != nil || (resp != nil && resp.IsError()) {
		t.Fatalf("failed to revoke credential: %v (resp: %v)", err, resp)
	}
}

func getCredentialFromStorage(t *testing.T, ctx context.Context, storage logical.Storage, username string) *models.Credential {
	t.Helper()

	entry, err := storage.Get(ctx, "creds/"+username)
	if err != nil {
		t.Fatalf("failed to get credential from storage: %v", err)
	}
	if entry == nil {
		return nil
	}

	var cred models.Credential
	if err := entry.DecodeJSON(&cred); err != nil {
		t.Fatalf("failed to decode credential: %v", err)
	}

	return &cred
}

func TestCredentialLifecycle(t *testing.T) {
	b, storage := setupTestBackend(t)
	ctx := context.Background()

	configureTestDatabase(t, b, storage)

	createTestRole(t, b, storage, testRoleName, 3600, 86400)

	username, password, ttl := generateTestCredential(t, b, storage, testRoleName)

	if username == "" {
		t.Fatal("generated username should not be empty")
	}
	if password == "" {
		t.Fatal("generated password should not be empty")
	}
	if ttl <= 0 {
		t.Fatal("generated TTL should be positive")
	}

	cred := getCredentialFromStorage(t, ctx, storage, username)
	if cred == nil {
		t.Fatal("credential should exist in storage after creation")
	}
	if cred.Username != username {
		t.Errorf("expected username %s, got %s", username, cred.Username)
	}
	if cred.RoleName != testRoleName {
		t.Errorf("expected role name %s, got %s", testRoleName, cred.RoleName)
	}
	if cred.ExpiresAt.IsZero() {
		t.Error("credential should have expiration time")
	}

	oldExpiresAt := cred.ExpiresAt
	oldLastRenewed := cred.LastRenewed

	newPassword, newTTL := renewTestCredential(t, b, storage, username)

	if newPassword == password {
		t.Error("renewed password should be different from old password")
	}
	if newPassword == "" {
		t.Fatal("renewed password should not be empty")
	}
	if newTTL <= 0 {
		t.Fatal("renewed TTL should be positive")
	}

	cred = getCredentialFromStorage(t, ctx, storage, username)
	if cred == nil {
		t.Fatal("credential should still exist in storage after renewal")
	}
	if cred.ExpiresAt.Unix() <= oldExpiresAt.Unix() {
		t.Error("expiration time should be extended after renewal")
	}
	if cred.LastRenewed.Unix() <= oldLastRenewed.Unix() {
		t.Error("last renewed time should be updated after renewal")
	}

	revokeTestCredential(t, b, storage, username)

	cred = getCredentialFromStorage(t, ctx, storage, username)
	if cred != nil {
		t.Error("credential should not exist in storage after revocation")
	}
}

func TestCredentialRenewalUpdatesExpiration(t *testing.T) {
	b, storage := setupTestBackend(t)
	ctx := context.Background()

	configureTestDatabase(t, b, storage)

	createTestRole(t, b, storage, "renew-test", 300, 1800)

	username, _, _ := generateTestCredential(t, b, storage, "renew-test")

	cred := getCredentialFromStorage(t, ctx, storage, username)
	originalExpiry := cred.ExpiresAt

	time.Sleep(1 * time.Second)

	_, _ = renewTestCredential(t, b, storage, username)

	cred = getCredentialFromStorage(t, ctx, storage, username)
	if !cred.ExpiresAt.After(originalExpiry) {
		t.Error("expiration should be extended after renewal")
	}

	revokeTestCredential(t, b, storage, username)
}

func TestCredentialRevokeRemovesFromStorage(t *testing.T) {
	b, storage := setupTestBackend(t)
	ctx := context.Background()

	configureTestDatabase(t, b, storage)

	createTestRole(t, b, storage, "revoke-test", 600, 3600)

	username, _, _ := generateTestCredential(t, b, storage, "revoke-test")

	cred := getCredentialFromStorage(t, ctx, storage, username)
	if cred == nil {
		t.Fatal("credential should exist before revocation")
	}

	revokeTestCredential(t, b, storage, username)

	cred = getCredentialFromStorage(t, ctx, storage, username)
	if cred != nil {
		t.Error("credential should be removed from storage after revocation")
	}
}

func TestCredentialCreateWithStatementTemplate(t *testing.T) {
	b, storage := setupTestBackend(t)

	configureTestDatabase(t, b, storage)

	ctx := context.Background()

	storage.Put(ctx, &logical.StorageEntry{
		Key:   "statements/test-template",
		Value: []byte(`{"name":"test-template","creation_statement":"GRANT SELECT ON DBC TO {{username}};","renewal_statement":"GRANT EXECUTE ON MyProc TO {{username}};","revocation_statement":"REVOKE SELECT ON DBC FROM {{username}};"}`),
	})

	createTestRoleWithTemplate(t, b, storage, "template-test", "test-template", 600, 3600)

	username, _, _ := generateTestCredential(t, b, storage, "template-test")

	revokeTestCredential(t, b, storage, username)
}

func createTestRoleWithTemplate(t *testing.T, b *Backend, storage logical.Storage, name, templateName string, defaultTTL, maxTTL int) {
	t.Helper()

	ctx := context.Background()

	data := map[string]interface{}{
		"name":               name,
		"db_user":            "{{username}}",
		"default_ttl":        defaultTTL,
		"max_ttl":            maxTTL,
		"statement_template": templateName,
	}

	req := &logical.Request{
		Operation: logical.CreateOperation,
		Path:      "roles/" + name,
		Storage:   storage,
		Data:      data,
	}

	resp, err := b.pathRoleCreate(ctx, req, &framework.FieldData{
		Raw:    data,
		Schema: b.pathRoles().Fields,
	})
	if err != nil || (resp != nil && resp.IsError()) {
		t.Fatalf("failed to create role with template: %v (resp: %v)", err, resp)
	}
}

func TestCredentialBatchLifecycle(t *testing.T) {
	b, storage := setupTestBackend(t)

	configureTestDatabase(t, b, storage)

	createTestRole(t, b, storage, "batch-test", 600, 3600)

	ctx := context.Background()

	data := map[string]interface{}{
		"name":  "batch-test",
		"count": 3,
	}

	req := &logical.Request{
		Operation: logical.ReadOperation,
		Path:      "creds/batch/batch-test",
		Storage:   storage,
		Data:      data,
	}

	resp, err := b.pathCredsBatchRead(ctx, req, &framework.FieldData{
		Raw:    data,
		Schema: b.pathCredsBatch().Fields,
	})
	if err != nil || (resp != nil && resp.IsError()) {
		t.Fatalf("failed to generate batch credentials: %v (resp: %v)", err, resp)
	}

	credentials := resp.Data["credentials"].([]map[string]interface{})
	if len(credentials) != 3 {
		t.Errorf("expected 3 credentials, got %d", len(credentials))
	}

	for i, cred := range credentials {
		username := cred["username"].(string)
		if username == "" {
			t.Errorf("credential %d username should not be empty", i)
		}
		if cred["password"].(string) == "" {
			t.Errorf("credential %d password should not be empty", i)
		}
		revokeTestCredential(t, b, storage, username)
	}
}

func TestCredentialRenewWithStatement(t *testing.T) {
	b, storage := setupTestBackend(t)

	configureTestDatabase(t, b, storage)

	createTestRoleWithRenewalStatement(t, b, storage, "renew-stmt-test", 600, 3600)

	username, _, _ := generateTestCredential(t, b, storage, "renew-stmt-test")

	_, _ = renewTestCredential(t, b, storage, username)

	revokeTestCredential(t, b, storage, username)
}

func createTestRoleWithRenewalStatement(t *testing.T, b *Backend, storage logical.Storage, name string, defaultTTL, maxTTL int) {
	t.Helper()

	ctx := context.Background()

	data := map[string]interface{}{
		"name":              name,
		"db_user":           "{{username}}",
		"default_ttl":       defaultTTL,
		"max_ttl":           maxTTL,
		"renewal_statement": "GRANT EXECUTE ON PROCEDURE MyProc TO {{username}};",
	}

	req := &logical.Request{
		Operation: logical.CreateOperation,
		Path:      "roles/" + name,
		Storage:   storage,
		Data:      data,
	}

	resp, err := b.pathRoleCreate(ctx, req, &framework.FieldData{
		Raw:    data,
		Schema: b.pathRoles().Fields,
	})
	if err != nil || (resp != nil && resp.IsError()) {
		t.Fatalf("failed to create role with renewal statement: %v (resp: %v)", err, resp)
	}
}

func TestCredentialLifecycleWithRevocationStatement(t *testing.T) {
	b, storage := setupTestBackend(t)

	configureTestDatabase(t, b, storage)

	createTestRole(t, b, storage, "revoke-stmt-test", 600, 3600)

	username, _, _ := generateTestCredential(t, b, storage, "revoke-stmt-test")

	revokeTestCredential(t, b, storage, username)
}
