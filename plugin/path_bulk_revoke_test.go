package teradata

import (
	"context"
	"testing"

	"github.com/openbao/openbao/sdk/v2/framework"
	"github.com/openbao/openbao/sdk/v2/logical"
)

func TestPathBulkRevokeCreds_Schema(t *testing.T) {
	b := NewBackend()
	path := b.pathBulkRevokeCreds()

	if path == nil {
		t.Fatal("pathBulkRevokeCreds returned nil")
	}

	if path.Pattern != "bulk-revoke-creds" {
		t.Errorf("expected pattern 'bulk-revoke-creds', got %q", path.Pattern)
	}

	usernameField, ok := path.Fields["usernames"]
	if !ok {
		t.Fatal("usernames field not found")
	}
	if usernameField.Type != framework.TypeStringSlice {
		t.Errorf("expected usernames field type TypeStringSlice, got %v", usernameField.Type)
	}
}

func TestPathBulkRevokeByRole_Schema(t *testing.T) {
	b := NewBackend()
	path := b.pathBulkRevokeByRole()

	if path == nil {
		t.Fatal("pathBulkRevokeByRole returned nil")
	}

	if path.Pattern != "bulk-revoke-creds/role/"+framework.GenericNameRegex("role") {
		t.Errorf("unexpected pattern %q", path.Pattern)
	}

	roleField, ok := path.Fields["role"]
	if !ok {
		t.Fatal("role field not found")
	}
	if roleField.Type != framework.TypeString {
		t.Errorf("expected role field type TypeString, got %v", roleField.Type)
	}
}

func TestPathRevokeAllCreds_Schema(t *testing.T) {
	b := NewBackend()
	path := b.pathRevokeAllCreds()

	if path == nil {
		t.Fatal("pathRevokeAllCreds returned nil")
	}

	if path.Pattern != "bulk-revoke-creds/all" {
		t.Errorf("expected pattern 'bulk-revoke-creds/all', got %q", path.Pattern)
	}
}

func TestBulkRevokeCredsHandler_MissingUsernames(t *testing.T) {
	b := NewBackend()

	storage := &testStorage{data: make(map[string][]byte)}

	fd := framework.FieldData{
		Raw:    map[string]interface{}{},
		Schema: map[string]*framework.FieldSchema{},
	}

	_, err := b.pathBulkRevokeCredsHandler(context.Background(), &logical.Request{
		Operation: logical.UpdateOperation,
		Path:      "bulk-revoke-creds",
		Storage:   storage,
		Data:      fd.Raw,
	}, &fd)
	if err == nil {
		t.Error("expected error for missing usernames parameter")
	}
}

func TestBulkRevokeByRoleHandler_MissingRole(t *testing.T) {
	b := NewBackend()

	storage := &testStorage{data: make(map[string][]byte)}

	fd := framework.FieldData{
		Raw: map[string]interface{}{
			"role": "",
		},
		Schema: map[string]*framework.FieldSchema{
			"role": {
				Type: framework.TypeString,
			},
		},
	}

	_, err := b.pathBulkRevokeByRoleHandler(context.Background(), &logical.Request{
		Operation: logical.DeleteOperation,
		Path:      "bulk-revoke-creds/role/testrole",
		Storage:   storage,
		Data:      fd.Raw,
	}, &fd)
	if err == nil {
		t.Error("expected error for missing role parameter")
	}
}

type testStorage struct {
	data map[string][]byte
}

func (s *testStorage) List(ctx context.Context, prefix string) ([]string, error) {
	var keys []string
	for k := range s.data {
		keys = append(keys, k)
	}
	return keys, nil
}

func (s *testStorage) Get(ctx context.Context, key string) (*logical.StorageEntry, error) {
	if val, ok := s.data[key]; ok {
		return &logical.StorageEntry{Key: key, Value: val}, nil
	}
	return nil, nil
}

func (s *testStorage) Put(ctx context.Context, entry *logical.StorageEntry) error {
	s.data[entry.Key] = entry.Value
	return nil
}

func (s *testStorage) Delete(ctx context.Context, key string) error {
	delete(s.data, key)
	return nil
}

func (s *testStorage) ListPage(ctx context.Context, prefix string, after string, limit int) ([]string, error) {
	return s.List(ctx, prefix)
}

func (s *testStorage) Keys(ctx context.Context, prefix string) ([]string, error) {
	return s.List(ctx, prefix)
}

func TestBulkRevokeResult_Structure(t *testing.T) {
	result := bulkRevokeResult{
		Username: "testuser",
		Success:  true,
		Error:    "",
	}

	if result.Username != "testuser" {
		t.Errorf("expected username 'testuser', got %q", result.Username)
	}
	if !result.Success {
		t.Error("expected success to be true")
	}
	if result.Error != "" {
		t.Errorf("expected empty error, got %q", result.Error)
	}
}

func TestBulkRevokeResponse_Structure(t *testing.T) {
	response := bulkRevokeResponse{
		Revoked: 2,
		Failed:  1,
		Results: []bulkRevokeResult{
			{Username: "user1", Success: true},
			{Username: "user2", Success: true},
			{Username: "user3", Success: false, Error: "not found"},
		},
	}

	if response.Revoked != 2 {
		t.Errorf("expected revoked 2, got %d", response.Revoked)
	}
	if response.Failed != 1 {
		t.Errorf("expected failed 1, got %d", response.Failed)
	}
	if len(response.Results) != 3 {
		t.Errorf("expected 3 results, got %d", len(response.Results))
	}
}

func TestReplaceVariables(t *testing.T) {
	tests := []struct {
		input    string
		username string
		password string
		expected string
	}{
		{"CREATE USER {{username}}", "testuser", "", "CREATE USER testuser"},
		{"PASSWORD = {{password}}", "", "secretpass", "PASSWORD = secretpass"},
		{"{{username}} AND {{password}}", "user", "pass", "user AND pass"},
		{"NO VARS", "user", "pass", "NO VARS"},
		{"{{username}}", "", "", "{{username}}"},
		{"{{password}}", "", "", "{{password}}"},
	}

	for _, tc := range tests {
		result := replaceVariables(tc.input, tc.username, tc.password)
		if result != tc.expected {
			t.Errorf("replaceVariables(%q, %q, %q) = %q, expected %q",
				tc.input, tc.username, tc.password, result, tc.expected)
		}
	}
}

func TestReplaceVariable(t *testing.T) {
	result := replaceVariable("Hello {{name}}", "{{name}}", "World")
	if result != "Hello World" {
		t.Errorf("expected 'Hello World', got %q", result)
	}

	result = replaceVariable("No placeholder here", "{{name}}", "World")
	if result != "No placeholder here" {
		t.Errorf("expected unchanged string, got %q", result)
	}

	result = replaceVariable("{{empty}}", "{{empty}}", "")
	if result != "{{empty}}" {
		t.Errorf("expected '{{empty}}' since empty value returns sql unchanged, got %q", result)
	}
}

func TestReplaceAllString(t *testing.T) {
	result := replaceAllString("aaa", "a", "b")
	if result != "bbb" {
		t.Errorf("expected 'bbb', got %q", result)
	}

	result = replaceAllString("aba", "a", "c")
	if result != "cbc" {
		t.Errorf("expected 'cbc', got %q", result)
	}

	result = replaceAllString("xyz", "a", "b")
	if result != "xyz" {
		t.Errorf("expected unchanged 'xyz', got %q", result)
	}
}
