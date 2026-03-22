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
		name                    string
		connectionString        string
		connectionURL           string
		maxOpenConnections      int
		maxIdleConnections      int
		connectionTimeout       int
		maxRetries              int
		initialRetryInterval    int
		maxRetryInterval        int
		retryMultiplier         float64
		sessionVariables        map[string]string
		timezone                string
		characterSet            string
		evictionPolicy          string
		evictionBatchSize       int
		evictionGracePeriod     int
		minEvictableIdleTime    int
		gracefulDegradationMode bool
		maxResultRows           int
		maxConnectionLifetime   int
		idleTimeout             int
		sessionTimeout          int
		sslMode                 string
		sslCert                 string
		sslKey                  string
		sslRootCert             string
		minConnections          int
		wantErr                 bool
		errContains             string
		checkResponse           func(*testing.T, *logical.Response)
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
				connDetails, ok := resp.Data["connection_details"].(map[string]interface{})
				if !ok {
					t.Fatal("connection_details is not a map")
				}
				if connDetails["connection_string"] != "***" {
					t.Errorf("expected connection_string to be masked, got %v", connDetails["connection_string"])
				}
				if connDetails["max_open_connections"] != 10 {
					t.Errorf("expected max_open_connections 10, got %v", connDetails["max_open_connections"])
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
			errContains:      "connection_string or connection_url is required",
		},
		{
			name:             "whitespace connection string",
			connectionString: "   ",
			wantErr:          true,
			errContains:      "connection",
		},
		{
			name:             "missing DSN and SERVER",
			connectionString: "UID=user;PWD=pass",
			wantErr:          true,
			errContains:      "connection",
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
		{
			name:             "valid config with connection_url",
			connectionString: "",
			connectionURL:    "teradata://user:pass@localhost:1025/dbc",
			wantErr:          false,
		},
		{
			name:               "valid config with SSL settings",
			connectionString:   "DSN=teradata;UID=user;PWD=pass",
			maxOpenConnections: 5,
			maxIdleConnections: 2,
			connectionTimeout:  30,
			sslMode:            "require",
			sslCert:            "/path/to/cert.pem",
			sslKey:             "/path/to/key.pem",
			sslRootCert:        "/path/to/root.pem",
			wantErr:            false,
		},
		{
			name:                 "valid config with retry settings",
			connectionString:     "DSN=teradata",
			maxOpenConnections:   5,
			maxIdleConnections:   2,
			connectionTimeout:    30,
			maxRetries:           5,
			initialRetryInterval: 200,
			maxRetryInterval:     6000,
			retryMultiplier:      2.5,
			wantErr:              false,
		},
		{
			name:             "valid config with session variables",
			connectionString: "DSN=teradata",
			sessionVariables: map[string]string{"SESSION_DATE": "ANSI", "MODE": "TERA"},
			wantErr:          false,
		},
		{
			name:             "valid config with timezone and charset",
			connectionString: "DSN=teradata",
			timezone:         "America/New_York",
			characterSet:     "utf8",
			wantErr:          false,
		},
		{
			name:                 "valid config with eviction policy",
			connectionString:     "DSN=teradata",
			evictionPolicy:       "fifo",
			evictionBatchSize:    5,
			evictionGracePeriod:  60,
			minEvictableIdleTime: 600,
			wantErr:              false,
		},
		{
			name:                    "valid config with graceful degradation",
			connectionString:        "DSN=teradata",
			gracefulDegradationMode: true,
			wantErr:                 false,
		},
		{
			name:             "valid config with max result rows",
			connectionString: "DSN=teradata",
			maxResultRows:    1000,
			wantErr:          false,
		},
		{
			name:                  "valid config with connection lifetime settings",
			connectionString:      "DSN=teradata",
			maxConnectionLifetime: 7200,
			idleTimeout:           600,
			sessionTimeout:        3600,
			wantErr:               false,
		},
		{
			name:             "invalid SSL mode",
			connectionString: "DSN=teradata",
			sslMode:          "invalid",
			wantErr:          true,
			errContains:      "ssl_mode",
		},
		{
			name:             "invalid eviction policy",
			connectionString: "DSN=teradata",
			evictionPolicy:   "invalid",
			wantErr:          true,
			errContains:      "eviction_policy",
		},
		{
			name:             "negative max retries",
			connectionString: "DSN=teradata",
			maxRetries:       -1,
			wantErr:          true,
			errContains:      "max_retries",
		},
		{
			name:                 "negative initial retry interval",
			connectionString:     "DSN=teradata",
			initialRetryInterval: -100,
			wantErr:              true,
			errContains:          "initial_retry_interval",
		},
		{
			name:             "negative max retry interval",
			connectionString: "DSN=teradata",
			maxRetryInterval: -500,
			wantErr:          true,
			errContains:      "max_retry_interval",
		},
		{
			name:             "negative retry multiplier",
			connectionString: "DSN=teradata",
			retryMultiplier:  -1.0,
			wantErr:          true,
			errContains:      "retry_multiplier",
		},
		{
			name:             "negative max result rows",
			connectionString: "DSN=teradata",
			maxResultRows:    -10,
			wantErr:          true,
			errContains:      "max_result_rows",
		},
		{
			name:              "negative eviction batch size",
			connectionString:  "DSN=teradata",
			evictionBatchSize: -1,
			wantErr:           true,
			errContains:       "eviction_batch_size",
		},
		{
			name:                "negative eviction grace period",
			connectionString:    "DSN=teradata",
			evictionGracePeriod: -30,
			wantErr:             true,
			errContains:         "eviction_grace_period",
		},
		{
			name:                 "negative min evictable idle time",
			connectionString:     "DSN=teradata",
			minEvictableIdleTime: -300,
			wantErr:              true,
			errContains:          "min_evictable_idle_time",
		},
		{
			name:               "max idle greater than max open",
			connectionString:   "DSN=teradata",
			maxOpenConnections: 5,
			maxIdleConnections: 10,
			wantErr:            true,
			errContains:        "max_idle_connections",
		},
		{
			name:               "max open less than min connections",
			connectionString:   "DSN=teradata",
			minConnections:     10,
			maxOpenConnections: 5,
			wantErr:            true,
			errContains:        "max_open_connections",
		},
		{
			name:             "negative min connections",
			connectionString: "DSN=teradata",
			minConnections:   -1,
			wantErr:          true,
			errContains:      "min_connections",
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
				"name":              "test",
				"plugin_name":       "teradata-database-plugin",
				"plugin_version":    "",
				"verify_connection": false,
				"allowed_roles":     []string{},
				"connection_string": tt.connectionString,
				"connection_url":    tt.connectionURL,
				"username":          "testuser",
				"password":          "testpass",
			}
			if tt.maxOpenConnections != 0 || tt.maxIdleConnections != 0 || tt.connectionTimeout != 0 {
				rawData["max_open_connections"] = tt.maxOpenConnections
				rawData["max_idle_connections"] = tt.maxIdleConnections
				rawData["connection_timeout"] = tt.connectionTimeout
			}
			if tt.connectionURL != "" {
				rawData["connection_url"] = tt.connectionURL
			}
			if tt.sslMode != "" {
				rawData["ssl_mode"] = tt.sslMode
			}
			if tt.sslCert != "" {
				rawData["ssl_cert"] = tt.sslCert
			}
			if tt.sslKey != "" {
				rawData["ssl_key"] = tt.sslKey
			}
			if tt.sslRootCert != "" {
				rawData["ssl_root_cert"] = tt.sslRootCert
			}
			if tt.maxRetries != 0 {
				rawData["max_retries"] = tt.maxRetries
			}
			if tt.initialRetryInterval != 0 {
				rawData["initial_retry_interval"] = tt.initialRetryInterval
			}
			if tt.maxRetryInterval != 0 {
				rawData["max_retry_interval"] = tt.maxRetryInterval
			}
			if tt.retryMultiplier != 0 {
				rawData["retry_multiplier"] = tt.retryMultiplier
			}
			if tt.sessionVariables != nil {
				rawData["session_variables"] = tt.sessionVariables
			}
			if tt.timezone != "" {
				rawData["timezone"] = tt.timezone
			}
			if tt.characterSet != "" {
				rawData["character_set"] = tt.characterSet
			}
			if tt.evictionPolicy != "" {
				rawData["eviction_policy"] = tt.evictionPolicy
			}
			if tt.evictionBatchSize != 0 {
				rawData["eviction_batch_size"] = tt.evictionBatchSize
			}
			if tt.evictionGracePeriod != 0 {
				rawData["eviction_grace_period"] = tt.evictionGracePeriod
			}
			if tt.minEvictableIdleTime != 0 {
				rawData["min_evictable_idle_time"] = tt.minEvictableIdleTime
			}
			if tt.gracefulDegradationMode {
				rawData["graceful_degradation_mode"] = tt.gracefulDegradationMode
			}
			if tt.maxResultRows != 0 {
				rawData["max_result_rows"] = tt.maxResultRows
			}
			if tt.maxConnectionLifetime != 0 {
				rawData["max_connection_lifetime"] = tt.maxConnectionLifetime
			}
			if tt.idleTimeout != 0 {
				rawData["idle_timeout"] = tt.idleTimeout
			}
			if tt.sessionTimeout != 0 {
				rawData["session_timeout"] = tt.sessionTimeout
			}
			if tt.minConnections != 0 {
				rawData["min_connections"] = tt.minConnections
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
				entry, err := logical.StorageEntryJSON("config/test", cfg)
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
				connDetails, ok := resp.Data["connection_details"].(map[string]interface{})
				if !ok {
					t.Fatal("connection_details is not a map")
				}
				if connDetails["connection_string"] != "***" {
					t.Errorf("expected connection_string masked, got %v", connDetails["connection_string"])
				}
				if connDetails["max_open_connections"] != 5 {
					t.Errorf("expected max_open_connections 5, got %v", connDetails["max_open_connections"])
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
				Raw:    map[string]interface{}{"name": "test"},
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
				entry, err := logical.StorageEntryJSON("config/test", cfg)
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
				Raw:    map[string]interface{}{"name": "test"},
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
		"name": {
			Type: framework.TypeString,
		},
		"region": {
			Type: framework.TypeString,
		},
		"plugin_name": {
			Type: framework.TypeString,
		},
		"plugin_version": {
			Type: framework.TypeString,
		},
		"verify_connection": {
			Type: framework.TypeBool,
		},
		"allowed_roles": {
			Type: framework.TypeStringSlice,
		},
		"root_rotation_statements": {
			Type: framework.TypeStringSlice,
		},
		"password_policy": {
			Type: framework.TypeString,
		},
		"connection_url": {
			Type: framework.TypeString,
		},
		"connection_string": {
			Type: framework.TypeString,
		},
		"connection_string_template": {
			Type: framework.TypeString,
		},
		"username": {
			Type: framework.TypeString,
		},
		"password": {
			Type: framework.TypeString,
		},
		"disable_escaping": {
			Type: framework.TypeBool,
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
		"session_timeout": {
			Type: framework.TypeInt,
		},
		"max_connection_lifetime": {
			Type: framework.TypeInt,
		},
		"idle_timeout": {
			Type: framework.TypeInt,
		},
		"max_retries": {
			Type: framework.TypeInt,
		},
		"initial_retry_interval": {
			Type: framework.TypeInt,
		},
		"max_retry_interval": {
			Type: framework.TypeInt,
		},
		"retry_multiplier": {
			Type: framework.TypeFloat,
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
		"session_variables": {
			Type: framework.TypeMap,
		},
		"graceful_degradation_mode": {
			Type:    framework.TypeBool,
			Default: false,
		},
		"max_result_rows": {
			Type:    framework.TypeInt,
			Default: 0,
		},
		"eviction_policy": {
			Type:    framework.TypeString,
			Default: "lifo",
		},
		"eviction_batch_size": {
			Type:    framework.TypeInt,
			Default: 1,
		},
		"eviction_grace_period": {
			Type:    framework.TypeInt,
			Default: 30,
		},
		"min_evictable_idle_time": {
			Type:    framework.TypeInt,
			Default: 300,
		},
		"timezone": {
			Type: framework.TypeString,
		},
		"character_set": {
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
