package teradata

import (
	"context"
	"testing"

	"github.com/JavierLimon/openbao-teradata-secret-plugin/security"
	"github.com/openbao/openbao/sdk/v2/framework"
	"github.com/openbao/openbao/sdk/v2/logical"
)

func TestPathConfigWrite(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name               string
		connectionString   string
		maxOpenConnections int
		maxIdleConnections int
		connectionTimeout  int
		wantErr            bool
		errContains        string
		checkResponse      func(*testing.T, *logical.Response)
	}{
		{
			name:               "valid config with DSN",
			connectionString:   "DSN=teradata;UID=user;PWD=pass",
			maxOpenConnections: 10,
			maxIdleConnections: 5,
			connectionTimeout:  30,
			wantErr:            false,
			checkResponse: func(t *testing.T, resp *logical.Response) {
				if resp == nil {
					t.Fatal("response is nil")
				}
				if resp.Data["connection_string"] != "***" {
					t.Errorf("expected connection_string to be masked, got %v", resp.Data["connection_string"])
				}
				if resp.Data["max_open_connections"] != 10 {
					t.Errorf("expected max_open_connections 10, got %v", resp.Data["max_open_connections"])
				}
			},
		},
		{
			name:               "valid config with SERVER",
			connectionString:   "SERVER=teradata.example.com;UID=user;PWD=pass",
			maxOpenConnections: 5,
			maxIdleConnections: 2,
			connectionTimeout:  60,
			wantErr:            false,
		},
		{
			name:             "empty connection string",
			connectionString: "",
			wantErr:          true,
			errContains:      "either connection_string or connection_string_template is required",
		},
		{
			name:             "whitespace connection string",
			connectionString: "   ",
			wantErr:          true,
			errContains:      "invalid connection string",
		},
		{
			name:             "missing DSN and SERVER",
			connectionString: "UID=user;PWD=pass",
			wantErr:          true,
			errContains:      "invalid connection string",
		},
		{
			name:             "valid with embedded semicolon in quoted value",
			connectionString: "DSN=teradata;PWD=\"pass;word\"",
			wantErr:          false,
		},
		{
			name:               "valid config with default values",
			connectionString:   "DSN=teradata",
			maxOpenConnections: 0,
			maxIdleConnections: 0,
			connectionTimeout:  0,
			wantErr:            false,
			checkResponse: func(t *testing.T, resp *logical.Response) {
				if resp.Data["max_open_connections"] != 0 {
					t.Errorf("expected default max_open_connections 0, got %v", resp.Data["max_open_connections"])
				}
			},
		},
		{
			name:               "zero connection timeout",
			connectionString:   "DSN=teradata",
			maxOpenConnections: 5,
			maxIdleConnections: 2,
			connectionTimeout:  0,
			wantErr:            false,
		},
		{
			name:             "valid with SERVERS",
			connectionString: "SERVERS=host1,host2;UID=user;PWD=pass",
			wantErr:          false,
		},
		{
			name:             "case insensitive DSN",
			connectionString: "dsn=mydsn;uid=user",
			wantErr:          false,
		},
		{
			name:             "valid complex connection string",
			connectionString: "DSN=Teradata;DBCNAME=teradata.example.com;UID=admin;PWD=secret123;charset=utf8",
			wantErr:          false,
		},
		{
			name:             "quoted values in connection string",
			connectionString: "DSN=teradata;PWD=\"my;complex;password\"",
			wantErr:          false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := NewBackend()
			ctx := context.Background()

			storage := &logical.InmemStorage{}
			req := &logical.Request{
				Storage: storage,
			}

			rawData := map[string]interface{}{
				"connection_string": tt.connectionString,
			}
			if tt.maxOpenConnections != 0 || tt.maxIdleConnections != 0 || tt.connectionTimeout != 0 {
				rawData["max_open_connections"] = tt.maxOpenConnections
				rawData["max_idle_connections"] = tt.maxIdleConnections
				rawData["connection_timeout"] = tt.connectionTimeout
			}

			data := &framework.FieldData{
				Raw:    rawData,
				Schema: getConfigFieldSchema(),
			}

			resp, err := b.pathConfigWrite(ctx, req, data)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
					return
				}
				if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("expected error containing %q, got %q", tt.errContains, err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if tt.checkResponse != nil {
				tt.checkResponse(t, resp)
			}
		})
	}
}

func TestPathConfigRead(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		setupStorage  func(logical.Storage) error
		wantErr       bool
		checkResponse func(*testing.T, *logical.Response)
	}{
		{
			name: "read existing config",
			setupStorage: func(storage logical.Storage) error {
				cfg := map[string]interface{}{
					"connection_string":    "DSN=teradata;UID=user;PWD=pass",
					"max_open_connections": 5,
					"max_idle_connections": 2,
					"connection_timeout":   30,
				}
				entry, err := logical.StorageEntryJSON("config", cfg)
				if err != nil {
					return err
				}
				return storage.Put(context.Background(), entry)
			},
			wantErr: false,
			checkResponse: func(t *testing.T, resp *logical.Response) {
				if resp == nil {
					t.Fatal("response is nil")
				}
				if resp.Data["connection_string"] != "***" {
					t.Errorf("expected connection_string masked, got %v", resp.Data["connection_string"])
				}
				if resp.Data["max_open_connections"] != 5 {
					t.Errorf("expected max_open_connections 5, got %v", resp.Data["max_open_connections"])
				}
			},
		},
		{
			name: "read non-existent config",
			setupStorage: func(storage logical.Storage) error {
				return nil
			},
			wantErr:       false,
			checkResponse: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := NewBackend()
			ctx := context.Background()

			storage := &logical.InmemStorage{}

			if err := tt.setupStorage(storage); err != nil {
				t.Fatalf("setup storage error: %v", err)
			}

			req := &logical.Request{
				Storage: storage,
			}

			data := &framework.FieldData{
				Raw:    map[string]interface{}{},
				Schema: getConfigFieldSchema(),
			}

			resp, err := b.pathConfigRead(ctx, req, data)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if tt.checkResponse != nil && resp != nil {
				tt.checkResponse(t, resp)
			}
		})
	}
}

func TestPathConfigDelete(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		setupStorage func(logical.Storage) error
		wantErr      bool
	}{
		{
			name: "delete existing config",
			setupStorage: func(storage logical.Storage) error {
				cfg := map[string]interface{}{
					"connection_string": "DSN=teradata",
				}
				entry, err := logical.StorageEntryJSON("config", cfg)
				if err != nil {
					return err
				}
				return storage.Put(context.Background(), entry)
			},
			wantErr: false,
		},
		{
			name: "delete non-existent config",
			setupStorage: func(storage logical.Storage) error {
				return nil
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := NewBackend()
			ctx := context.Background()

			storage := &logical.InmemStorage{}

			if err := tt.setupStorage(storage); err != nil {
				t.Fatalf("setup storage error: %v", err)
			}

			req := &logical.Request{
				Storage: storage,
			}

			data := &framework.FieldData{
				Raw:    map[string]interface{}{},
				Schema: getConfigFieldSchema(),
			}

			resp, err := b.pathConfigDelete(ctx, req, data)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if resp != nil {
				t.Error("expected nil response on successful delete")
			}
		})
	}
}

func TestConfigConnectionStringValidation(t *testing.T) {
	t.Parallel()

	invalidConnectionStrings := []string{
		"",
		"   ",
		"\t\n",
		"UID=user;PWD=pass",
		"key with space=value",
	}

	for _, connStr := range invalidConnectionStrings {
		t.Run("invalid_"+connStr, func(t *testing.T) {
			err := security.ValidateConnectionString(connStr)
			if err == nil {
				t.Errorf("ValidateConnectionString(%q) expected error, got nil", connStr)
			}
		})
	}

	validConnectionStrings := []string{
		"DSN=teradata",
		"DSN=teradata;UID=user;PWD=pass",
		"SERVER=localhost",
		"SERVER=localhost;UID=user;PWD=pass",
		"SERVERS=host1,host2",
		"dsn=mydsn",
		"Server=myserver",
		"DSN=Teradata_Prod;UID=admin;PWD=password;",
	}

	for _, connStr := range validConnectionStrings {
		t.Run("valid_"+connStr, func(t *testing.T) {
			err := security.ValidateConnectionString(connStr)
			if err != nil {
				t.Errorf("ValidateConnectionString(%q) unexpected error: %v", connStr, err)
			}
		})
	}
}

func getConfigFieldSchema() map[string]*framework.FieldSchema {
	return map[string]*framework.FieldSchema{
		"region": {
			Type: framework.TypeString,
		},
		"connection_string": {
			Type: framework.TypeString,
		},
		"connection_string_template": {
			Type: framework.TypeString,
		},
		"server": {
			Type: framework.TypeString,
		},
		"servers": {
			Type: framework.TypeString,
		},
		"port": {
			Type: framework.TypeInt,
		},
		"database": {
			Type: framework.TypeString,
		},
		"username": {
			Type: framework.TypeString,
		},
		"password": {
			Type: framework.TypeString,
		},
		"min_connections": {
			Type: framework.TypeInt,
		},
		"max_open_connections": {
			Type: framework.TypeInt,
		},
		"max_idle_connections": {
			Type: framework.TypeInt,
		},
		"connection_timeout": {
			Type: framework.TypeInt,
		},
		"max_connection_lifetime": {
			Type: framework.TypeInt,
		},
		"idle_timeout": {
			Type: framework.TypeInt,
		},
		"ssl_mode": {
			Type: framework.TypeString,
		},
		"ssl_cert": {
			Type: framework.TypeString,
		},
		"ssl_key": {
			Type: framework.TypeString,
		},
		"ssl_root_cert": {
			Type: framework.TypeString,
		},
		"ssl_key_password": {
			Type: framework.TypeString,
		},
		"ssl_cipher_suites": {
			Type: framework.TypeString,
		},
		"ssl_secure": {
			Type: framework.TypeBool,
		},
		"ssl_version": {
			Type: framework.TypeString,
		},
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
