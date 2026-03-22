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
	integrationTestRoleName = "integration-test-role"
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

type integrationStorage struct {
	mu   sync.RWMutex
	data map[string][]byte
}

func newIntegrationStorage() *integrationStorage {
	return &integrationStorage{
		data: make(map[string][]byte),
	}
}

func (s *integrationStorage) Put(ctx context.Context, entry *logical.StorageEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[entry.Key] = entry.Value
	return nil
}

func (s *integrationStorage) Get(ctx context.Context, key string) (*logical.StorageEntry, error) {
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

func (s *integrationStorage) Delete(ctx context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, key)
	return nil
}

func (s *integrationStorage) List(ctx context.Context, prefix string) ([]string, error) {
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

func (s *integrationStorage) ListPage(ctx context.Context, prefix string, after string, limit int) ([]string, error) {
	return s.List(ctx, prefix)
}

func (s *integrationStorage) Keys(ctx context.Context, prefix string) ([]string, error) {
	return s.List(ctx, prefix)
}

func setupIntegrationBackend(t *testing.T) (*Backend, logical.Storage) {
	t.Helper()

	storage := newIntegrationStorage()
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

func configureIntegrationDatabase(t *testing.T, b *Backend, storage logical.Storage) {
	t.Helper()

	ctx := context.Background()

	data := map[string]interface{}{
		"connection_string":      getTestConnectionString(),
		"max_open_connections":   5,
		"max_idle_connections":   2,
		"connection_timeout":     30,
		"query_timeout":          300,
		"session_timeout":        300,
		"max_retries":            3,
		"initial_retry_interval": 100,
		"max_retry_interval":     5000,
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

func createIntegrationRole(t *testing.T, b *Backend, storage logical.Storage, name string, defaultTTL, maxTTL int) {
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

func generateIntegrationCredential(t *testing.T, b *Backend, storage logical.Storage, roleName string) (string, string, int) {
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

func renewIntegrationCredential(t *testing.T, b *Backend, storage logical.Storage, username string) (string, int) {
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

func revokeIntegrationCredential(t *testing.T, b *Backend, storage logical.Storage, username string) {
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

func getIntegrationCredentialFromStorage(t *testing.T, ctx context.Context, storage logical.Storage, username string) *models.Credential {
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

func TestIntegrationCredentialCreateUser(t *testing.T) {
	if os.Getenv("RUN_INTEGRATION_TESTS") != "true" {
		t.Skip("Skipping integration test; set RUN_INTEGRATION_TESTS=true")
	}

	b, storage := setupIntegrationBackend(t)
	ctx := context.Background()

	configureIntegrationDatabase(t, b, storage)

	roleName := "create-user-test"
	createIntegrationRole(t, b, storage, roleName, 3600, 86400)

	username, password, ttl := generateIntegrationCredential(t, b, storage, roleName)

	if username == "" {
		t.Fatal("generated username should not be empty")
	}
	if password == "" {
		t.Fatal("generated password should not be empty")
	}
	if ttl <= 0 {
		t.Fatal("generated TTL should be positive")
	}

	cred := getIntegrationCredentialFromStorage(t, ctx, storage, username)
	if cred == nil {
		t.Fatal("credential should exist in storage after creation")
	}
	if cred.Username != username {
		t.Errorf("expected username %s, got %s", username, cred.Username)
	}
	if cred.RoleName != roleName {
		t.Errorf("expected role name %s, got %s", roleName, cred.RoleName)
	}
	if cred.ExpiresAt.IsZero() {
		t.Error("credential should have expiration time")
	}

	t.Logf("Successfully created Teradata user: %s with TTL: %d", username, ttl)

	revokeIntegrationCredential(t, b, storage, username)

	cred = getIntegrationCredentialFromStorage(t, ctx, storage, username)
	if cred != nil {
		t.Error("credential should not exist in storage after revocation")
	}
}

func TestIntegrationCredentialGrantFlow(t *testing.T) {
	if os.Getenv("RUN_INTEGRATION_TESTS") != "true" {
		t.Skip("Skipping integration test; set RUN_INTEGRATION_TESTS=true")
	}

	b, storage := setupIntegrationBackend(t)
	ctx := context.Background()

	configureIntegrationDatabase(t, b, storage)

	roleName := "grant-test"
	data := map[string]interface{}{
		"name":                 roleName,
		"db_user":              "{{username}}",
		"default_ttl":          3600,
		"max_ttl":              86400,
		"default_database":     roleName,
		"creation_statement":   "GRANT SELECT ON DBC TO {{username}};",
		"revocation_statement": "REVOKE SELECT ON DBC FROM {{username}};",
	}

	req := &logical.Request{
		Operation: logical.CreateOperation,
		Path:      "roles/" + roleName,
		Storage:   storage,
		Data:      data,
	}

	_, err := b.pathRoleCreate(ctx, req, &framework.FieldData{
		Raw:    data,
		Schema: b.pathRoles().Fields,
	})
	if err != nil || (req.Response != nil && req.Response.IsError()) {
		t.Fatalf("failed to create role: %v", err)
	}

	username, _, _ := generateIntegrationCredential(t, b, storage, roleName)

	t.Logf("Created user with GRANT: %s", username)

	revokeIntegrationCredential(t, b, storage, username)
}

func TestIntegrationCredentialRevokeFlow(t *testing.T) {
	if os.Getenv("RUN_INTEGRATION_TESTS") != "true" {
		t.Skip("Skipping integration test; set RUN_INTEGRATION_TESTS=true")
	}

	b, storage := setupIntegrationBackend(t)
	ctx := context.Background()

	configureIntegrationDatabase(t, b, storage)

	roleName := "revoke-test"
	createIntegrationRole(t, b, storage, roleName, 600, 3600)

	username, _, _ := generateIntegrationCredential(t, b, storage, roleName)

	cred := getIntegrationCredentialFromStorage(t, ctx, storage, username)
	if cred == nil {
		t.Fatal("credential should exist before revocation")
	}

	revokeIntegrationCredential(t, b, storage, username)

	cred = getIntegrationCredentialFromStorage(t, ctx, storage, username)
	if cred != nil {
		t.Error("credential should be removed from storage after revocation")
	}

	t.Logf("Successfully revoked and dropped user: %s", username)
}

func TestIntegrationCredentialDropUser(t *testing.T) {
	if os.Getenv("RUN_INTEGRATION_TESTS") != "true" {
		t.Skip("Skipping integration test; set RUN_INTEGRATION_TESTS=true")
	}

	b, storage := setupIntegrationBackend(t)
	ctx := context.Background()

	configureIntegrationDatabase(t, b, storage)

	roleName := "drop-user-test"
	createIntegrationRole(t, b, storage, roleName, 600, 3600)

	username, _, _ := generateIntegrationCredential(t, b, storage, roleName)

	cred := getIntegrationCredentialFromStorage(t, ctx, storage, username)
	if cred == nil {
		t.Fatal("credential should exist before DROP USER")
	}

	revokeIntegrationCredential(t, b, storage, username)

	cred = getIntegrationCredentialFromStorage(t, ctx, storage, username)
	if cred != nil {
		t.Error("credential should be removed after DROP USER")
	}

	t.Logf("Successfully dropped Teradata user: %s", username)
}

func TestIntegrationCredentialRenewal(t *testing.T) {
	if os.Getenv("RUN_INTEGRATION_TESTS") != "true" {
		t.Skip("Skipping integration test; set RUN_INTEGRATION_TESTS=true")
	}

	b, storage := setupIntegrationBackend(t)
	ctx := context.Background()

	configureIntegrationDatabase(t, b, storage)

	roleName := "renewal-test"
	createIntegrationRole(t, b, storage, roleName, 300, 1800)

	username, _, _ := generateIntegrationCredential(t, b, storage, roleName)

	cred := getIntegrationCredentialFromStorage(t, ctx, storage, username)
	originalExpiry := cred.ExpiresAt
	originalLastRenewed := cred.LastRenewed

	time.Sleep(1 * time.Second)

	newPassword, newTTL := renewIntegrationCredential(t, b, storage, username)

	if newPassword == "" {
		t.Fatal("renewed password should not be empty")
	}
	if newTTL <= 0 {
		t.Fatal("renewed TTL should be positive")
	}

	cred = getIntegrationCredentialFromStorage(t, ctx, storage, username)
	if !cred.ExpiresAt.After(originalExpiry) {
		t.Error("expiration should be extended after renewal")
	}
	if !cred.LastRenewed.After(originalLastRenewed) {
		t.Error("last renewed time should be updated after renewal")
	}

	t.Logf("Successfully renewed credential for user: %s, new TTL: %d", username, newTTL)

	revokeIntegrationCredential(t, b, storage, username)
}

func TestIntegrationCredentialFullLifecycle(t *testing.T) {
	if os.Getenv("RUN_INTEGRATION_TESTS") != "true" {
		t.Skip("Skipping integration test; set RUN_INTEGRATION_TESTS=true")
	}

	b, storage := setupIntegrationBackend(t)
	ctx := context.Background()

	configureIntegrationDatabase(t, b, storage)

	roleName := "full-lifecycle-test"
	createIntegrationRole(t, b, storage, roleName, 600, 3600)

	username, password, ttl := generateIntegrationCredential(t, b, storage, roleName)

	if username == "" || password == "" || ttl <= 0 {
		t.Fatal("initial credential generation failed")
	}

	cred := getIntegrationCredentialFromStorage(t, ctx, storage, username)
	if cred == nil {
		t.Fatal("credential should exist in storage after creation")
	}

	oldExpiresAt := cred.ExpiresAt

	time.Sleep(1 * time.Second)

	newPassword, newTTL := renewIntegrationCredential(t, b, storage, username)

	if newPassword == password {
		t.Error("renewed password should be different")
	}
	if newTTL <= 0 {
		t.Fatal("renewed TTL should be positive")
	}

	cred = getIntegrationCredentialFromStorage(t, ctx, storage, username)
	if !cred.ExpiresAt.After(oldExpiresAt) {
		t.Error("expiration should be extended after renewal")
	}

	revokeIntegrationCredential(t, b, storage, username)

	cred = getIntegrationCredentialFromStorage(t, ctx, storage, username)
	if cred != nil {
		t.Error("credential should not exist after revocation")
	}

	t.Logf("Full credential lifecycle completed for user: %s", username)
}

func TestIntegrationCredentialBatchLifecycle(t *testing.T) {
	if os.Getenv("RUN_INTEGRATION_TESTS") != "true" {
		t.Skip("Skipping integration test; set RUN_INTEGRATION_TESTS=true")
	}

	b, storage := setupIntegrationBackend(t)

	configureIntegrationDatabase(t, b, storage)

	roleName := "batch-lifecycle-test"
	createIntegrationRole(t, b, storage, roleName, 600, 3600)

	ctx := context.Background()

	data := map[string]interface{}{
		"name":  roleName,
		"count": 3,
	}

	req := &logical.Request{
		Operation: logical.ReadOperation,
		Path:      "creds/batch/" + roleName,
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
		t.Logf("Batch credential %d: %s", i, username)
	}

	for _, cred := range credentials {
		if username, ok := cred["username"].(string); ok {
			revokeIntegrationCredential(t, b, storage, username)
		}
	}

	t.Logf("Successfully created and revoked %d batch credentials", len(credentials))
}

func TestIntegrationCredentialWithStatementTemplate(t *testing.T) {
	if os.Getenv("RUN_INTEGRATION_TESTS") != "true" {
		t.Skip("Skipping integration test; set RUN_INTEGRATION_TESTS=true")
	}

	b, storage := setupIntegrationBackend(t)

	configureIntegrationDatabase(t, b, storage)

	ctx := context.Background()

	storage.Put(ctx, &logical.StorageEntry{
		Key:   "statements/test-template",
		Value: []byte(`{"name":"test-template","creation_statement":"GRANT SELECT ON DBC TO {{username}};","renewal_statement":"GRANT EXECUTE ON MyProc TO {{username}};","revocation_statement":"REVOKE SELECT ON DBC FROM {{username}};"}`),
	})

	roleName := "template-test"
	data := map[string]interface{}{
		"name":               roleName,
		"db_user":            "{{username}}",
		"default_ttl":        600,
		"max_ttl":            3600,
		"statement_template": "test-template",
	}

	req := &logical.Request{
		Operation: logical.CreateOperation,
		Path:      "roles/" + roleName,
		Storage:   storage,
		Data:      data,
	}

	_, err := b.pathRoleCreate(ctx, req, &framework.FieldData{
		Raw:    data,
		Schema: b.pathRoles().Fields,
	})
	if err != nil || (req.Response != nil && req.Response.IsError()) {
		t.Fatalf("failed to create role with template: %v", err)
	}

	username, _, _ := generateIntegrationCredential(t, b, storage, roleName)

	t.Logf("Created user with statement template: %s", username)

	revokeIntegrationCredential(t, b, storage, username)
}

func TestIntegrationCredentialRenewalWithStatement(t *testing.T) {
	if os.Getenv("RUN_INTEGRATION_TESTS") != "true" {
		t.Skip("Skipping integration test; set RUN_INTEGRATION_TESTS=true")
	}

	b, storage := setupIntegrationBackend(t)

	configureIntegrationDatabase(t, b, storage)

	roleName := "renew-stmt-test"
	data := map[string]interface{}{
		"name":              roleName,
		"db_user":           "{{username}}",
		"default_ttl":       600,
		"max_ttl":           3600,
		"renewal_statement": "GRANT EXECUTE ON PROCEDURE MyProc TO {{username}};",
	}

	ctx := context.Background()

	req := &logical.Request{
		Operation: logical.CreateOperation,
		Path:      "roles/" + roleName,
		Storage:   storage,
		Data:      data,
	}

	_, err := b.pathRoleCreate(ctx, req, &framework.FieldData{
		Raw:    data,
		Schema: b.pathRoles().Fields,
	})
	if err != nil || (req.Response != nil && req.Response.IsError()) {
		t.Fatalf("failed to create role with renewal statement: %v", err)
	}

	username, _, _ := generateIntegrationCredential(t, b, storage, roleName)

	_, _ = renewIntegrationCredential(t, b, storage, username)

	t.Logf("Successfully renewed credential with statement for: %s", username)

	revokeIntegrationCredential(t, b, storage, username)
}
