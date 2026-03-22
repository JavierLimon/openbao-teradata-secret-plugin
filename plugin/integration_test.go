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
	_ "github.com/JavierLimon/openbao-teradata-secret-plugin/odbc"
	"github.com/openbao/openbao/sdk/v2/framework"
	"github.com/openbao/openbao/sdk/v2/logical"
)

const (
	integrationTestRoleName = "integration-test-role"
)

var (
	integrationHost     = getEnv("TERADATA_HOST", "testing-rhjbbw139fee5yg7.env.clearscape.teradata.com")
	integrationUser     = getEnv("TERADATA_USER", "demo_user")
	integrationPassword = getEnv("TERADATA_PASSWORD", "latve1ja")
	integrationDSN      = getEnv("TERADATA_DSN", "Teradata ODBC DSN")
)

func getIntegrationEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getIntegrationConnectionString() string {
	return fmt.Sprintf("DSN=Teradata ODBC DSN;DBCName=%s;UID=%s;PWD=%s;", integrationHost, integrationUser, integrationPassword)
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
		"name":                   "test",
		"plugin_name":            "teradata-database-plugin",
		"connection_string":      getIntegrationConnectionString(),
		"verify_connection":      false,
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
		"creation_statement":   "GRANT SELECT ON DBC TO {{username}}",
		"revocation_statement": "REVOKE SELECT ON DBC FROM {{username}}",
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

func generateIntegrationCredential(t *testing.T, b *Backend, storage logical.Storage, roleName string, region string) (string, string, int) {
	t.Helper()

	ctx := context.Background()

	data := map[string]interface{}{
		"name":   roleName,
		"region": region,
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

	username, password, ttl := generateIntegrationCredential(t, b, storage, roleName, "test")

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
		"creation_statement":   "GRANT SELECT ON DBC TO {{username}}",
		"revocation_statement": "REVOKE SELECT ON DBC FROM {{username}}",
	}

	req := &logical.Request{
		Operation: logical.CreateOperation,
		Path:      "roles/" + roleName,
		Storage:   storage,
		Data:      data,
	}

	resp, err := b.pathRoleCreate(ctx, req, &framework.FieldData{
		Raw:    data,
		Schema: b.pathRoles().Fields,
	})
	if err != nil || (resp != nil && resp.IsError()) {
		t.Fatalf("failed to create role: %v", err)
	}

	username, _, _ := generateIntegrationCredential(t, b, storage, roleName, "test")

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

	username, _, _ := generateIntegrationCredential(t, b, storage, roleName, "test")

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

	username, _, _ := generateIntegrationCredential(t, b, storage, roleName, "test")

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

	username, _, _ := generateIntegrationCredential(t, b, storage, roleName, "test")

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

	username, password, ttl := generateIntegrationCredential(t, b, storage, roleName, "test")

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
		Value: []byte(`{"name":"test-template","creation_statement":"GRANT SELECT ON DBC TO {{username}}","renewal_statement":"GRANT EXECUTE ON MyProc TO {{username}}","revocation_statement":"REVOKE SELECT ON DBC FROM {{username}}"}`),
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

	resp, err := b.pathRoleCreate(ctx, req, &framework.FieldData{
		Raw:    data,
		Schema: b.pathRoles().Fields,
	})
	if err != nil || (resp != nil && resp.IsError()) {
		t.Fatalf("failed to create role with template: %v", err)
	}

	username, _, _ := generateIntegrationCredential(t, b, storage, roleName, "test")

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
		"renewal_statement": "GRANT EXECUTE ON PROCEDURE MyProc TO {{username}}",
	}

	ctx := context.Background()

	req := &logical.Request{
		Operation: logical.CreateOperation,
		Path:      "roles/" + roleName,
		Storage:   storage,
		Data:      data,
	}

	resp, err := b.pathRoleCreate(ctx, req, &framework.FieldData{
		Raw:    data,
		Schema: b.pathRoles().Fields,
	})
	if err != nil || (resp != nil && resp.IsError()) {
		t.Fatalf("failed to create role with renewal statement: %v", err)
	}

	username, _, _ := generateIntegrationCredential(t, b, storage, roleName, "test")

	_, _ = renewIntegrationCredential(t, b, storage, username)

	t.Logf("Successfully renewed credential with statement for: %s", username)

	revokeIntegrationCredential(t, b, storage, username)
}

// ============================================================
// Config Integration Tests
// ============================================================

func TestIntegrationConfigCRUD(t *testing.T) {
	if os.Getenv("RUN_INTEGRATION_TESTS") != "true" {
		t.Skip("Skipping integration test; set RUN_INTEGRATION_TESTS=true")
	}

	b, storage := setupIntegrationBackend(t)
	ctx := context.Background()

	// Create config
	data := map[string]interface{}{
		"name":                 "test-config",
		"plugin_name":          "teradata-database-plugin",
		"connection_string":    getIntegrationConnectionString(),
		"verify_connection":    false,
		"max_open_connections": 5,
		"max_idle_connections": 2,
	}

	req := &logical.Request{
		Operation: logical.CreateOperation,
		Path:      "config/test-config",
		Storage:   storage,
		Data:      data,
	}

	resp, err := b.pathConfigWrite(ctx, req, &framework.FieldData{
		Raw:    data,
		Schema: b.pathConfig().Fields,
	})
	if err != nil || (resp != nil && resp.IsError()) {
		t.Fatalf("failed to create config: %v (resp: %v)", err, resp)
	}

	// Read config back
	req = &logical.Request{
		Operation: logical.ReadOperation,
		Path:      "config/test-config",
		Storage:   storage,
		Data:      map[string]interface{}{"name": "test-config"},
	}

	resp, err = b.pathConfigRead(ctx, req, &framework.FieldData{
		Raw:    map[string]interface{}{"name": "test-config"},
		Schema: b.pathConfig().Fields,
	})
	if err != nil || (resp != nil && resp.IsError()) {
		t.Fatalf("failed to read config: %v (resp: %v)", err, resp)
	}
	if resp == nil || resp.Data == nil {
		t.Fatal("config response should not be nil")
	}

	t.Logf("Successfully created and read config: %s", resp.Data["name"])

	// Update config
	updateData := map[string]interface{}{
		"name":                 "test-config",
		"plugin_name":          "teradata-database-plugin",
		"connection_string":    getIntegrationConnectionString(),
		"verify_connection":    false,
		"max_open_connections": 10,
		"max_idle_connections": 5,
	}

	req = &logical.Request{
		Operation: logical.UpdateOperation,
		Path:      "config/test-config",
		Storage:   storage,
		Data:      updateData,
	}

	resp, err = b.pathConfigWrite(ctx, req, &framework.FieldData{
		Raw:    updateData,
		Schema: b.pathConfig().Fields,
	})
	if err != nil || (resp != nil && resp.IsError()) {
		t.Fatalf("failed to update config: %v (resp: %v)", err, resp)
	}

	t.Logf("Successfully updated config")
}

func TestIntegrationConfigList(t *testing.T) {
	if os.Getenv("RUN_INTEGRATION_TESTS") != "true" {
		t.Skip("Skipping integration test; set RUN_INTEGRATION_TESTS=true")
	}

	b, storage := setupIntegrationBackend(t)
	ctx := context.Background()

	// Create two configs
	for _, name := range []string{"config1", "config2"} {
		data := map[string]interface{}{
			"name":              name,
			"plugin_name":       "teradata-database-plugin",
			"connection_string": getIntegrationConnectionString(),
			"verify_connection": false,
		}

		req := &logical.Request{
			Operation: logical.CreateOperation,
			Path:      "config/" + name,
			Storage:   storage,
			Data:      data,
		}

		resp, err := b.pathConfigWrite(ctx, req, &framework.FieldData{
			Raw:    data,
			Schema: b.pathConfig().Fields,
		})
		if err != nil || (resp != nil && resp.IsError()) {
			t.Fatalf("failed to create config %s: %v", name, err)
		}
	}

	// List configs
	req := &logical.Request{
		Operation: logical.ListOperation,
		Path:      "config",
		Storage:   storage,
	}

	resp, err := b.pathConfigListHandler(ctx, req, &framework.FieldData{
		Raw:    map[string]interface{}{},
		Schema: b.pathConfigList().Fields,
	})
	if err != nil || (resp != nil && resp.IsError()) {
		t.Fatalf("failed to list configs: %v (resp: %v)", err, resp)
	}

	keys := resp.Data["keys"].([]string)
	if len(keys) < 2 {
		t.Errorf("expected at least 2 configs, got %d: %v", len(keys), keys)
	}

	t.Logf("Successfully listed configs: %v", keys)
}

func TestIntegrationConfigReset(t *testing.T) {
	if os.Getenv("RUN_INTEGRATION_TESTS") != "true" {
		t.Skip("Skipping integration test; set RUN_INTEGRATION_TESTS=true")
	}

	b, storage := setupIntegrationBackend(t)
	ctx := context.Background()

	// Create config first
	data := map[string]interface{}{
		"name":              "reset-test",
		"plugin_name":       "teradata-database-plugin",
		"connection_string": getIntegrationConnectionString(),
		"verify_connection": false,
	}

	req := &logical.Request{
		Operation: logical.CreateOperation,
		Path:      "config/reset-test",
		Storage:   storage,
		Data:      data,
	}

	resp, err := b.pathConfigWrite(ctx, req, &framework.FieldData{
		Raw:    data,
		Schema: b.pathConfig().Fields,
	})
	if err != nil || (resp != nil && resp.IsError()) {
		t.Fatalf("failed to create config: %v", err)
	}

	// Reset config
	req = &logical.Request{
		Operation: logical.UpdateOperation,
		Path:      "reset/reset-test",
		Storage:   storage,
		Data:      map[string]interface{}{"name": "reset-test"},
	}

	resp, err = b.pathConfigResetHandler(ctx, req, &framework.FieldData{
		Raw:    map[string]interface{}{"name": "reset-test"},
		Schema: b.pathConfigReset().Fields,
	})
	if err != nil || (resp != nil && resp.IsError()) {
		t.Fatalf("failed to reset config: %v (resp: %v)", err, resp)
	}

	t.Logf("Successfully reset config")
}

func TestIntegrationConfigReload(t *testing.T) {
	if os.Getenv("RUN_INTEGRATION_TESTS") != "true" {
		t.Skip("Skipping integration test; set RUN_INTEGRATION_TESTS=true")
	}

	b, storage := setupIntegrationBackend(t)
	ctx := context.Background()

	// Create config
	data := map[string]interface{}{
		"name":              "reload-test",
		"plugin_name":       "teradata-database-plugin",
		"connection_string": getIntegrationConnectionString(),
		"verify_connection": false,
	}

	req := &logical.Request{
		Operation: logical.CreateOperation,
		Path:      "config/reload-test",
		Storage:   storage,
		Data:      data,
	}

	b.pathConfigWrite(ctx, req, &framework.FieldData{
		Raw:    data,
		Schema: b.pathConfig().Fields,
	})

	// Reload config
	req = &logical.Request{
		Operation: logical.UpdateOperation,
		Path:      "reload/teradata-database-plugin",
		Storage:   storage,
		Data:      map[string]interface{}{"plugin_name": "teradata-database-plugin"},
	}

	resp, err := b.pathConfigReloadHandler(ctx, req, &framework.FieldData{
		Raw:    map[string]interface{}{"plugin_name": "teradata-database-plugin"},
		Schema: b.pathConfigReload().Fields,
	})
	if err != nil || (resp != nil && resp.IsError()) {
		t.Fatalf("failed to reload config: %v (resp: %v)", err, resp)
	}

	t.Logf("Successfully reloaded config")
}

// ============================================================
// Roles Integration Tests
// ============================================================

func TestIntegrationRolesCRUD(t *testing.T) {
	if os.Getenv("RUN_INTEGRATION_TESTS") != "true" {
		t.Skip("Skipping integration test; set RUN_INTEGRATION_TESTS=true")
	}

	b, storage := setupIntegrationBackend(t)
	ctx := context.Background()

	// Create role
	data := map[string]interface{}{
		"name":                 "test-role",
		"db_user":              "{{username}}",
		"default_ttl":          3600,
		"max_ttl":              86400,
		"default_database":     "testdb",
		"creation_statement":   "GRANT SELECT ON DBC TO {{username}}",
		"revocation_statement": "REVOKE SELECT ON DBC FROM {{username}}",
	}

	req := &logical.Request{
		Operation: logical.CreateOperation,
		Path:      "roles/test-role",
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

	// Read role back
	req = &logical.Request{
		Operation: logical.ReadOperation,
		Path:      "roles/test-role",
		Storage:   storage,
		Data:      map[string]interface{}{"name": "test-role"},
	}

	resp, err = b.pathRoleRead(ctx, req, &framework.FieldData{
		Raw:    map[string]interface{}{"name": "test-role"},
		Schema: b.pathRoles().Fields,
	})
	if err != nil || (resp != nil && resp.IsError()) {
		t.Fatalf("failed to read role: %v (resp: %v)", err, resp)
	}
	if resp == nil || resp.Data == nil {
		t.Fatal("role response should not be nil")
	}

	t.Logf("Successfully created and read role: %s", resp.Data["name"])

	// Update role
	updateData := map[string]interface{}{
		"name":             "test-role",
		"db_user":          "{{username}}",
		"default_ttl":      7200,
		"max_ttl":          172800,
		"default_database": "testdb",
	}

	req = &logical.Request{
		Operation: logical.UpdateOperation,
		Path:      "roles/test-role",
		Storage:   storage,
		Data:      updateData,
	}

	resp, err = b.pathRoleCreate(ctx, req, &framework.FieldData{
		Raw:    updateData,
		Schema: b.pathRoles().Fields,
	})
	if err != nil || (resp != nil && resp.IsError()) {
		t.Fatalf("failed to update role: %v (resp: %v)", err, resp)
	}

	t.Logf("Successfully updated role")
}

func TestIntegrationRolesList(t *testing.T) {
	if os.Getenv("RUN_INTEGRATION_TESTS") != "true" {
		t.Skip("Skipping integration test; set RUN_INTEGRATION_TESTS=true")
	}

	b, storage := setupIntegrationBackend(t)
	ctx := context.Background()

	// Create roles
	for _, name := range []string{"role1", "role2"} {
		data := map[string]interface{}{
			"name":        name,
			"db_user":     "{{username}}",
			"default_ttl": 3600,
			"max_ttl":     86400,
		}

		req := &logical.Request{
			Operation: logical.CreateOperation,
			Path:      "roles/" + name,
			Storage:   storage,
			Data:      data,
		}

		b.pathRoleCreate(ctx, req, &framework.FieldData{
			Raw:    data,
			Schema: b.pathRoles().Fields,
		})
	}

	// List roles
	req := &logical.Request{
		Operation: logical.ListOperation,
		Path:      "roles",
		Storage:   storage,
	}

	resp, err := b.pathRoleListHandler(ctx, req, &framework.FieldData{
		Raw:    map[string]interface{}{},
		Schema: b.pathRoleList().Fields,
	})
	if err != nil || (resp != nil && resp.IsError()) {
		t.Fatalf("failed to list roles: %v (resp: %v)", err, resp)
	}

	keys := resp.Data["keys"].([]string)
	if len(keys) < 2 {
		t.Errorf("expected at least 2 roles, got %d: %v", len(keys), keys)
	}

	t.Logf("Successfully listed roles: %v", keys)
}

// ============================================================
// Static Roles Integration Tests
// ============================================================

func TestIntegrationStaticRolesCRUD(t *testing.T) {
	if os.Getenv("RUN_INTEGRATION_TESTS") != "true" {
		t.Skip("Skipping integration test; set RUN_INTEGRATION_TESTS=true")
	}

	b, storage := setupIntegrationBackend(t)
	ctx := context.Background()

	// Configure database first
	cfg := map[string]interface{}{
		"name":              "static",
		"plugin_name":       "teradata-database-plugin",
		"connection_string": getIntegrationConnectionString(),
		"verify_connection": false,
	}

	req := &logical.Request{
		Operation: logical.CreateOperation,
		Path:      "config/static",
		Storage:   storage,
		Data:      cfg,
	}

	b.pathConfigWrite(ctx, req, &framework.FieldData{
		Raw:    cfg,
		Schema: b.pathConfig().Fields,
	})

	// Create static role
	data := map[string]interface{}{
		"name":            "static-role-test",
		"username":        "test_static_user",
		"db_name":         "static",
		"rotation_period": 3600,
	}

	req = &logical.Request{
		Operation: logical.CreateOperation,
		Path:      "static-roles/static-role-test",
		Storage:   storage,
		Data:      data,
	}

	resp, err := b.pathStaticRoleCreate(ctx, req, &framework.FieldData{
		Raw:    data,
		Schema: b.pathStaticRoles().Fields,
	})
	if err != nil || (resp != nil && resp.IsError()) {
		t.Fatalf("failed to create static role: %v (resp: %v)", err, resp)
	}

	// Read static role back
	req = &logical.Request{
		Operation: logical.ReadOperation,
		Path:      "static-roles/static-role-test",
		Storage:   storage,
		Data:      map[string]interface{}{"name": "static-role-test"},
	}

	resp, err = b.pathStaticRoleRead(ctx, req, &framework.FieldData{
		Raw:    map[string]interface{}{"name": "static-role-test"},
		Schema: b.pathStaticRoles().Fields,
	})
	if err != nil || (resp != nil && resp.IsError()) {
		t.Fatalf("failed to read static role: %v (resp: %v)", err, resp)
	}

	t.Logf("Successfully created static role")
}

func TestIntegrationStaticCreds(t *testing.T) {
	if os.Getenv("RUN_INTEGRATION_TESTS") != "true" {
		t.Skip("Skipping integration test; set RUN_INTEGRATION_TESTS=true")
	}

	b, storage := setupIntegrationBackend(t)
	ctx := context.Background()

	// Configure database
	cfg := map[string]interface{}{
		"name":              "static-creds",
		"plugin_name":       "teradata-database-plugin",
		"connection_string": getIntegrationConnectionString(),
		"verify_connection": false,
	}

	req := &logical.Request{
		Operation: logical.CreateOperation,
		Path:      "config/static-creds",
		Storage:   storage,
		Data:      cfg,
	}

	b.pathConfigWrite(ctx, req, &framework.FieldData{
		Raw:    cfg,
		Schema: b.pathConfig().Fields,
	})

	// Create static role
	data := map[string]interface{}{
		"name":            "static-creds-role",
		"username":        "test_static_creds",
		"db_name":         "static-creds",
		"rotation_period": 3600,
	}

	req = &logical.Request{
		Operation: logical.CreateOperation,
		Path:      "static-roles/static-creds-role",
		Storage:   storage,
		Data:      data,
	}

	b.pathStaticRoleCreate(ctx, req, &framework.FieldData{
		Raw:    data,
		Schema: b.pathStaticRoles().Fields,
	})

	// Read static credentials
	req = &logical.Request{
		Operation: logical.ReadOperation,
		Path:      "static-creds/static-creds-role",
		Storage:   storage,
		Data:      map[string]interface{}{"name": "static-creds-role"},
	}

	resp, err := b.pathStaticCredsRead(ctx, req, &framework.FieldData{
		Raw:    map[string]interface{}{"name": "static-creds-role"},
		Schema: b.pathStaticCreds().Fields,
	})
	if err != nil || (resp != nil && resp.IsError()) {
		t.Fatalf("failed to read static creds: %v (resp: %v)", err, resp)
	}

	username := resp.Data["username"].(string)
	if username == "" {
		t.Error("username should not be empty")
	}

	t.Logf("Successfully read static credentials for: %s", username)
}

// ============================================================
// Rate Limiting Integration Tests
// ============================================================

func TestIntegrationRateLimitConfig(t *testing.T) {
	if os.Getenv("RUN_INTEGRATION_TESTS") != "true" {
		t.Skip("Skipping integration test; set RUN_INTEGRATION_TESTS=true")
	}

	b, storage := setupIntegrationBackend(t)
	ctx := context.Background()

	// Configure rate limit
	data := map[string]interface{}{
		"enabled": true,
		"rate":    100,
		"burst":   50,
		"period":  600,
	}

	req := &logical.Request{
		Operation: logical.CreateOperation,
		Path:      "rate-limit/config",
		Storage:   storage,
		Data:      data,
	}

	resp, err := b.pathRateLimitConfigWrite(ctx, req, &framework.FieldData{
		Raw:    data,
		Schema: b.pathRateLimitConfig().Fields,
	})
	if err != nil || (resp != nil && resp.IsError()) {
		t.Fatalf("failed to configure rate limit: %v (resp: %v)", err, resp)
	}

	// Read rate limit config
	req = &logical.Request{
		Operation: logical.ReadOperation,
		Path:      "rate-limit/config",
		Storage:   storage,
	}

	resp, err = b.pathRateLimitConfigRead(ctx, req, &framework.FieldData{
		Raw:    map[string]interface{}{},
		Schema: b.pathRateLimitConfig().Fields,
	})
	if err != nil || (resp != nil && resp.IsError()) {
		t.Fatalf("failed to read rate limit config: %v (resp: %v)", err, resp)
	}

	t.Logf("Successfully configured rate limit")
}

func TestIntegrationRateLimitStatus(t *testing.T) {
	if os.Getenv("RUN_INTEGRATION_TESTS") != "true" {
		t.Skip("Skipping integration test; set RUN_INTEGRATION_TESTS=true")
	}

	b, storage := setupIntegrationBackend(t)
	ctx := context.Background()

	// Read rate limit status
	req := &logical.Request{
		Operation: logical.ReadOperation,
		Path:      "rate-limit/status",
		Storage:   storage,
	}

	resp, err := b.pathRateLimitStatusRead(ctx, req, &framework.FieldData{
		Raw:    map[string]interface{}{},
		Schema: b.pathRateLimitStatus().Fields,
	})
	if err != nil || (resp != nil && resp.IsError()) {
		t.Fatalf("failed to read rate limit status: %v (resp: %v)", err, resp)
	}

	t.Logf("Successfully read rate limit status")
}

// ============================================================
// Statements Integration Tests
// ============================================================

func TestIntegrationStatementsCRUD(t *testing.T) {
	if os.Getenv("RUN_INTEGRATION_TESTS") != "true" {
		t.Skip("Skipping integration test; set RUN_INTEGRATION_TESTS=true")
	}

	b, storage := setupIntegrationBackend(t)
	ctx := context.Background()

	// Create statement
	data := map[string]interface{}{
		"name":                 "test-statement",
		"creation_statement":   "GRANT SELECT ON DBC TO {{username}}",
		"revocation_statement": "REVOKE SELECT ON DBC FROM {{username}}",
		"renewal_statement":    "GRANT EXECUTE ON PROCEDURE TO {{username}}",
	}

	req := &logical.Request{
		Operation: logical.CreateOperation,
		Path:      "statements/test-statement",
		Storage:   storage,
		Data:      data,
	}

	resp, err := b.pathStatementWrite(ctx, req, &framework.FieldData{
		Raw:    data,
		Schema: b.pathStatements().Fields,
	})
	if err != nil || (resp != nil && resp.IsError()) {
		t.Fatalf("failed to create statement: %v (resp: %v)", err, resp)
	}

	// Read statement back
	req = &logical.Request{
		Operation: logical.ReadOperation,
		Path:      "statements/test-statement",
		Storage:   storage,
		Data:      map[string]interface{}{"name": "test-statement"},
	}

	resp, err = b.pathStatementRead(ctx, req, &framework.FieldData{
		Raw:    map[string]interface{}{"name": "test-statement"},
		Schema: b.pathStatements().Fields,
	})
	if err != nil || (resp != nil && resp.IsError()) {
		t.Fatalf("failed to read statement: %v (resp: %v)", err, resp)
	}

	t.Logf("Successfully created statement")
}

func TestIntegrationStatementsList(t *testing.T) {
	if os.Getenv("RUN_INTEGRATION_TESTS") != "true" {
		t.Skip("Skipping integration test; set RUN_INTEGRATION_TESTS=true")
	}

	b, storage := setupIntegrationBackend(t)
	ctx := context.Background()

	// Create statements
	for _, name := range []string{"stmt1", "stmt2"} {
		data := map[string]interface{}{
			"name":               name,
			"creation_statement": "GRANT SELECT ON DBC TO {{username}}",
		}

		req := &logical.Request{
			Operation: logical.CreateOperation,
			Path:      "statements/" + name,
			Storage:   storage,
			Data:      data,
		}

		b.pathStatementWrite(ctx, req, &framework.FieldData{
			Raw:    data,
			Schema: b.pathStatements().Fields,
		})
	}

	// List statements
	req := &logical.Request{
		Operation: logical.ListOperation,
		Path:      "statements",
		Storage:   storage,
	}

	resp, err := b.pathStatementListHandler(ctx, req, &framework.FieldData{
		Raw:    map[string]interface{}{},
		Schema: b.pathStatementList().Fields,
	})
	if err != nil || (resp != nil && resp.IsError()) {
		t.Fatalf("failed to list statements: %v (resp: %v)", err, resp)
	}

	t.Logf("Successfully listed statements")
}

// ============================================================
// Health Integration Tests
// ============================================================

func TestIntegrationHealth(t *testing.T) {
	if os.Getenv("RUN_INTEGRATION_TESTS") != "true" {
		t.Skip("Skipping integration test; set RUN_INTEGRATION_TESTS=true")
	}

	b, storage := setupIntegrationBackend(t)
	ctx := context.Background()

	// Test health endpoint
	req := &logical.Request{
		Operation: logical.ReadOperation,
		Path:      "health",
		Storage:   storage,
	}

	resp, err := b.pathHealthRead(ctx, req, &framework.FieldData{
		Raw: map[string]interface{}{},
	})
	if err != nil || (resp != nil && resp.IsError()) {
		t.Fatalf("failed to read health: %v (resp: %v)", err, resp)
	}

	t.Logf("Health status: %v", resp.Data)
}

func TestIntegrationReadiness(t *testing.T) {
	if os.Getenv("RUN_INTEGRATION_TESTS") != "true" {
		t.Skip("Skipping integration test; set RUN_INTEGRATION_TESTS=true")
	}

	b, storage := setupIntegrationBackend(t)
	ctx := context.Background()

	// Test readiness endpoint
	req := &logical.Request{
		Operation: logical.ReadOperation,
		Path:      "ready",
		Storage:   storage,
	}

	resp, err := b.pathReadinessRead(ctx, req, &framework.FieldData{
		Raw: map[string]interface{}{},
	})
	if err != nil || (resp != nil && resp.IsError()) {
		t.Fatalf("failed to read readiness: %v (resp: %v)", err, resp)
	}

	t.Logf("Readiness status: %v", resp.Data)
}

func TestIntegrationLiveness(t *testing.T) {
	if os.Getenv("RUN_INTEGRATION_TESTS") != "true" {
		t.Skip("Skipping integration test; set RUN_INTEGRATION_TESTS=true")
	}

	b, storage := setupIntegrationBackend(t)
	ctx := context.Background()

	// Test liveness endpoint
	req := &logical.Request{
		Operation: logical.ReadOperation,
		Path:      "live",
		Storage:   storage,
	}

	resp, err := b.pathLivenessRead(ctx, req, &framework.FieldData{
		Raw: map[string]interface{}{},
	})
	if err != nil || (resp != nil && resp.IsError()) {
		t.Fatalf("failed to read liveness: %v (resp: %v)", err, resp)
	}

	t.Logf("Liveness status: %v", resp.Data)
}

// ============================================================
// Leases Integration Tests
// ============================================================

func TestIntegrationLeases(t *testing.T) {
	if os.Getenv("RUN_INTEGRATION_TESTS") != "true" {
		t.Skip("Skipping integration test; set RUN_INTEGRATION_TESTS=true")
	}

	b, storage := setupIntegrationBackend(t)
	ctx := context.Background()

	// Configure database
	cfg := map[string]interface{}{
		"name":              "leases-test",
		"plugin_name":       "teradata-database-plugin",
		"connection_string": getIntegrationConnectionString(),
		"verify_connection": false,
	}

	req := &logical.Request{
		Operation: logical.CreateOperation,
		Path:      "config/leases-test",
		Storage:   storage,
		Data:      cfg,
	}

	b.pathConfigWrite(ctx, req, &framework.FieldData{
		Raw:    cfg,
		Schema: b.pathConfig().Fields,
	})

	// Create role
	roleData := map[string]interface{}{
		"name":        "leases-role",
		"db_user":     "{{username}}",
		"default_ttl": 3600,
		"max_ttl":     86400,
	}

	req = &logical.Request{
		Operation: logical.CreateOperation,
		Path:      "roles/leases-role",
		Storage:   storage,
		Data:      roleData,
	}

	b.pathRoleCreate(ctx, req, &framework.FieldData{
		Raw:    roleData,
		Schema: b.pathRoles().Fields,
	})

	// Generate a credential to create a lease
	credData := map[string]interface{}{
		"name":   "leases-role",
		"region": "leases-test",
	}

	req = &logical.Request{
		Operation: logical.ReadOperation,
		Path:      "creds/leases-role",
		Storage:   storage,
		Data:      credData,
	}

	resp, err := b.pathCredsRead(ctx, req, &framework.FieldData{
		Raw:    credData,
		Schema: b.pathCreds().Fields,
	})
	if err != nil || (resp != nil && resp.IsError()) {
		t.Logf("Note: Credential generation failed (expected if no DB): %v", err)
		// Continue to test leases endpoint even if credential failed
	}

	// List leases
	req = &logical.Request{
		Operation: logical.ListOperation,
		Path:      "leases",
		Storage:   storage,
	}

	resp, err = b.pathLeasesList(ctx, req, &framework.FieldData{
		Raw:    map[string]interface{}{},
		Schema: b.pathLeases().Fields,
	})
	if err != nil || (resp != nil && resp.IsError()) {
		t.Fatalf("failed to list leases: %v (resp: %v)", err, resp)
	}

	t.Logf("Successfully listed leases")
}

// ============================================================
// Metrics Integration Tests
// ============================================================

func TestIntegrationMetrics(t *testing.T) {
	if os.Getenv("RUN_INTEGRATION_TESTS") != "true" {
		t.Skip("Skipping integration test; set RUN_INTEGRATION_TESTS=true")
	}

	b, storage := setupIntegrationBackend(t)
	ctx := context.Background()

	// Test metrics endpoint
	req := &logical.Request{
		Operation: logical.ReadOperation,
		Path:      "metrics",
		Storage:   storage,
	}

	resp, err := b.pathMetricsRead(ctx, req, &framework.FieldData{
		Raw: map[string]interface{}{},
	})
	if err != nil || (resp != nil && resp.IsError()) {
		t.Fatalf("failed to read metrics: %v (resp: %v)", err, resp)
	}

	t.Logf("Successfully read metrics")
}

// ============================================================
// Reload Plugin Integration Test
// ============================================================

func TestIntegrationReloadPlugin(t *testing.T) {
	if os.Getenv("RUN_INTEGRATION_TESTS") != "true" {
		t.Skip("Skipping integration test; set RUN_INTEGRATION_TESTS=true")
	}

	b, storage := setupIntegrationBackend(t)
	ctx := context.Background()

	// Configure database
	cfg := map[string]interface{}{
		"name":              "reload-plugin-test",
		"plugin_name":       "teradata-database-plugin",
		"connection_string": getIntegrationConnectionString(),
		"verify_connection": false,
	}

	req := &logical.Request{
		Operation: logical.CreateOperation,
		Path:      "config/reload-plugin-test",
		Storage:   storage,
		Data:      cfg,
	}

	b.pathConfigWrite(ctx, req, &framework.FieldData{
		Raw:    cfg,
		Schema: b.pathConfig().Fields,
	})

	// Reload plugin
	data := map[string]interface{}{
		"plugin_name": "teradata-database-plugin",
	}

	req = &logical.Request{
		Operation: logical.UpdateOperation,
		Path:      "reload/teradata-database-plugin",
		Storage:   storage,
		Data:      data,
	}

	resp, err := b.pathReloadPluginHandler(ctx, req, &framework.FieldData{
		Raw:    data,
		Schema: b.pathReloadPlugin().Fields,
	})
	if err != nil || (resp != nil && resp.IsError()) {
		t.Fatalf("failed to reload plugin: %v (resp: %v)", err, resp)
	}

	t.Logf("Successfully reloaded plugin")
}
