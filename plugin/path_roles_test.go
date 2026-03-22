package teradata

import (
	"context"
	"testing"

	"github.com/JavierLimon/openbao-teradata-secret-plugin/models"
	"github.com/openbao/openbao/sdk/v2/framework"
	"github.com/openbao/openbao/sdk/v2/logical"
)

func TestPathRoleCreate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		roleName      string
		dbUser        string
		dbPassword    string
		defaultTTL    int
		maxTTL        int
		wantErr       bool
		checkResponse func(*testing.T, *logical.Response)
		checkStorage  func(*testing.T, logical.Storage)
	}{
		{
			name:       "valid role creation",
			roleName:   "test-role",
			dbUser:     "user{{username}}",
			dbPassword: "pass{{password}}",
			defaultTTL: 3600,
			maxTTL:     86400,
			wantErr:    false,
			checkResponse: func(t *testing.T, resp *logical.Response) {
				if resp == nil {
					t.Fatal("response is nil")
				}
				if resp.Data["name"] != "test-role" {
					t.Errorf("expected name 'test-role', got %v", resp.Data["name"])
				}
				if resp.Data["db_user"] != "user{{username}}" {
					t.Errorf("expected db_user 'user{{username}}', got %v", resp.Data["db_user"])
				}
				if resp.Data["default_ttl"] != 3600 {
					t.Errorf("expected default_ttl 3600, got %v", resp.Data["default_ttl"])
				}
				if resp.Data["max_ttl"] != 86400 {
					t.Errorf("expected max_ttl 86400, got %v", resp.Data["max_ttl"])
				}
			},
			checkStorage: func(t *testing.T, storage logical.Storage) {
				entry, err := storage.Get(context.Background(), "roles/test-role")
				if err != nil {
					t.Errorf("error retrieving role: %v", err)
					return
				}
				if entry == nil {
					t.Error("role not found in storage")
					return
				}
				var role models.Role
				if err := entry.DecodeJSON(&role); err != nil {
					t.Errorf("error decoding role: %v", err)
					return
				}
				if role.Name != "test-role" {
					t.Errorf("expected role name 'test-role', got %s", role.Name)
				}
			},
		},
		{
			name:       "role with empty TTL values uses defaults",
			roleName:   "role-defaults",
			dbUser:     "user{{username}}",
			dbPassword: "password",
			defaultTTL: 0,
			maxTTL:     0,
			wantErr:    false,
			checkResponse: func(t *testing.T, resp *logical.Response) {
				if resp.Data["default_ttl"] != 0 {
					t.Errorf("expected default_ttl 0, got %v", resp.Data["default_ttl"])
				}
				if resp.Data["max_ttl"] != 0 {
					t.Errorf("expected max_ttl 0, got %v", resp.Data["max_ttl"])
				}
			},
		},
		{
			name:       "role with creation statement",
			roleName:   "with-creation-stmt",
			dbUser:     "user{{username}}",
			dbPassword: "password",
			wantErr:    false,
			checkResponse: func(t *testing.T, resp *logical.Response) {
				if resp.Data["creation_statement"] != "***" {
					t.Errorf("expected creation_statement to be masked, got %v", resp.Data["creation_statement"])
				}
			},
		},
		{
			name:       "role with statement template",
			roleName:   "with-template",
			dbUser:     "user{{username}}",
			dbPassword: "password",
			wantErr:    false,
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

			data := &framework.FieldData{
				Raw: map[string]interface{}{
					"name":               tt.roleName,
					"db_user":            tt.dbUser,
					"db_password":        tt.dbPassword,
					"default_ttl":        tt.defaultTTL,
					"max_ttl":            tt.maxTTL,
					"creation_statement": "GRANT SELECT ON mydb TO {{username}}",
				},
				Schema: getRoleFieldSchema(),
			}

			resp, err := b.pathRoleCreate(ctx, req, data)

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

			if tt.checkResponse != nil {
				tt.checkResponse(t, resp)
			}

			if tt.checkStorage != nil {
				tt.checkStorage(t, storage)
			}
		})
	}
}

func TestPathRoleRead(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		setupStorage  func(logical.Storage) error
		roleName      string
		wantErr       bool
		checkResponse func(*testing.T, *logical.Response)
	}{
		{
			name:     "read existing role",
			roleName: "existing-role",
			setupStorage: func(storage logical.Storage) error {
				role := &models.Role{
					Name:                "existing-role",
					DBUser:              "user{{username}}",
					DBPassword:          "secret123",
					DefaultTTL:          3600,
					MaxTTL:              86400,
					StatementTemplate:   "template1",
					CreationStatement:   "CREATE USER",
					RevocationStatement: "DROP USER",
				}
				entry, err := logical.StorageEntryJSON("roles/existing-role", role)
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
				if resp.Data["name"] != "existing-role" {
					t.Errorf("expected name 'existing-role', got %v", resp.Data["name"])
				}
				if resp.Data["db_user"] != "user{{username}}" {
					t.Errorf("expected db_user 'user{{username}}', got %v", resp.Data["db_user"])
				}
				if resp.Data["db_password"] != "***" {
					t.Errorf("expected password to be masked, got %v", resp.Data["db_password"])
				}
				if resp.Data["default_ttl"] != 3600 {
					t.Errorf("expected default_ttl 3600, got %v", resp.Data["default_ttl"])
				}
				if resp.Data["statement_template"] != "template1" {
					t.Errorf("expected statement_template 'template1', got %v", resp.Data["statement_template"])
				}
			},
		},
		{
			name:     "read non-existent role",
			roleName: "nonexistent",
			setupStorage: func(storage logical.Storage) error {
				return nil
			},
			wantErr: false,
			checkResponse: func(t *testing.T, resp *logical.Response) {
				if resp != nil {
					t.Error("expected nil response for non-existent role")
				}
			},
		},
		{
			name:     "read role without password",
			roleName: "no-password-role",
			setupStorage: func(storage logical.Storage) error {
				role := &models.Role{
					Name:       "no-password-role",
					DBUser:     "user{{username}}",
					DefaultTTL: 3600,
					MaxTTL:     86400,
				}
				entry, err := logical.StorageEntryJSON("roles/no-password-role", role)
				if err != nil {
					return err
				}
				return storage.Put(context.Background(), entry)
			},
			wantErr: false,
			checkResponse: func(t *testing.T, resp *logical.Response) {
				if _, ok := resp.Data["db_password"]; ok {
					t.Error("expected no db_password field when password is empty")
				}
			},
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
				Raw: map[string]interface{}{
					"name": tt.roleName,
				},
				Schema: getRoleFieldSchema(),
			}

			resp, err := b.pathRoleRead(ctx, req, data)

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

			if tt.checkResponse != nil {
				tt.checkResponse(t, resp)
			}
		})
	}
}

func TestPathRoleUpdate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		setupStorage  func(logical.Storage) error
		roleName      string
		newDBUser     string
		newDefaultTTL int
		newMaxTTL     int
		wantErr       bool
		checkResponse func(*testing.T, *logical.Response)
		checkStorage  func(*testing.T, logical.Storage)
	}{
		{
			name:     "update existing role",
			roleName: "update-test",
			setupStorage: func(storage logical.Storage) error {
				role := &models.Role{
					Name:       "update-test",
					DBUser:     "old{{username}}",
					DBPassword: "oldpass",
					DefaultTTL: 1800,
					MaxTTL:     3600,
				}
				entry, err := logical.StorageEntryJSON("roles/update-test", role)
				if err != nil {
					return err
				}
				return storage.Put(context.Background(), entry)
			},
			newDBUser:     "new{{username}}",
			newDefaultTTL: 7200,
			newMaxTTL:     14400,
			wantErr:       false,
			checkResponse: func(t *testing.T, resp *logical.Response) {
				if resp.Data["db_user"] != "new{{username}}" {
					t.Errorf("expected db_user 'new{{username}}', got %v", resp.Data["db_user"])
				}
			},
			checkStorage: func(t *testing.T, storage logical.Storage) {
				entry, err := storage.Get(context.Background(), "roles/update-test")
				if err != nil {
					t.Errorf("error retrieving role: %v", err)
					return
				}
				var role models.Role
				if err := entry.DecodeJSON(&role); err != nil {
					t.Errorf("error decoding role: %v", err)
					return
				}
				if role.DBUser != "new{{username}}" {
					t.Errorf("storage: expected db_user 'new{{username}}', got %s", role.DBUser)
				}
				if role.DefaultTTL != 7200 {
					t.Errorf("storage: expected default_ttl 7200, got %d", role.DefaultTTL)
				}
			},
		},
		{
			name:     "update non-existent role creates new",
			roleName: "nonexistent",
			setupStorage: func(storage logical.Storage) error {
				return nil
			},
			newDBUser: "user{{username}}",
			wantErr:   false,
			checkStorage: func(t *testing.T, storage logical.Storage) {
				entry, err := storage.Get(context.Background(), "roles/nonexistent")
				if err != nil {
					t.Errorf("error retrieving role: %v", err)
					return
				}
				if entry == nil {
					t.Error("expected role to be created")
					return
				}
				var role models.Role
				if err := entry.DecodeJSON(&role); err != nil {
					t.Errorf("error decoding role: %v", err)
					return
				}
				if role.DBUser != "user{{username}}" {
					t.Errorf("expected db_user 'user{{username}}', got %s", role.DBUser)
				}
			},
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
				Raw: map[string]interface{}{
					"name":            tt.roleName,
					"db_user":         tt.newDBUser,
					"db_password":     "newpass",
					"default_ttl":     tt.newDefaultTTL,
					"max_ttl":         tt.newMaxTTL,
					"max_credentials": 10,
				},
				Schema: getRoleFieldSchema(),
			}

			resp, err := b.pathRoleUpdate(ctx, req, data)

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

			if tt.checkResponse != nil {
				tt.checkResponse(t, resp)
			}

			if tt.checkStorage != nil {
				tt.checkStorage(t, storage)
			}
		})
	}
}

func TestPathRoleDelete(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		setupStorage func(logical.Storage) error
		roleName     string
		wantErr      bool
		checkDeleted func(*testing.T, logical.Storage)
	}{
		{
			name:     "delete existing role",
			roleName: "delete-me",
			setupStorage: func(storage logical.Storage) error {
				role := &models.Role{
					Name:       "delete-me",
					DBUser:     "user{{username}}",
					DBPassword: "password",
				}
				entry, err := logical.StorageEntryJSON("roles/delete-me", role)
				if err != nil {
					return err
				}
				return storage.Put(context.Background(), entry)
			},
			wantErr: false,
			checkDeleted: func(t *testing.T, storage logical.Storage) {
				entry, err := storage.Get(context.Background(), "roles/delete-me")
				if err != nil {
					t.Errorf("error checking storage: %v", err)
					return
				}
				if entry != nil {
					t.Error("expected role to be deleted, but it still exists")
				}
			},
		},
		{
			name:     "delete non-existent role succeeds",
			roleName: "nonexistent",
			setupStorage: func(storage logical.Storage) error {
				return nil
			},
			wantErr: false,
			checkDeleted: func(t *testing.T, storage logical.Storage) {
				entry, err := storage.Get(context.Background(), "roles/nonexistent")
				if err != nil {
					t.Errorf("error checking storage: %v", err)
					return
				}
				if entry != nil {
					t.Error("expected no entry for non-existent role")
				}
			},
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
				Raw: map[string]interface{}{
					"name": tt.roleName,
				},
				Schema: getRoleFieldSchema(),
			}

			resp, err := b.pathRoleDelete(ctx, req, data)

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

			if tt.checkDeleted != nil {
				tt.checkDeleted(t, storage)
			}
		})
	}
}

func TestPathRoleList(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		setupStorage  func(logical.Storage) error
		wantErr       bool
		checkResponse func(*testing.T, *logical.Response)
	}{
		{
			name: "list multiple roles",
			setupStorage: func(storage logical.Storage) error {
				roles := []string{"role1", "role2", "role3"}
				for _, name := range roles {
					role := &models.Role{
						Name:   name,
						DBUser: "user{{username}}",
					}
					entry, err := logical.StorageEntryJSON("roles/"+name, role)
					if err != nil {
						return err
					}
					if err := storage.Put(context.Background(), entry); err != nil {
						return err
					}
				}
				return nil
			},
			wantErr: false,
			checkResponse: func(t *testing.T, resp *logical.Response) {
				if resp == nil {
					t.Fatal("response is nil")
				}
				keys, ok := resp.Data["keys"].([]string)
				if !ok {
					t.Fatal("keys not found in response")
				}
				if len(keys) != 3 {
					t.Errorf("expected 3 keys, got %d", len(keys))
				}
			},
		},
		{
			name: "list empty roles",
			setupStorage: func(storage logical.Storage) error {
				return nil
			},
			wantErr: false,
			checkResponse: func(t *testing.T, resp *logical.Response) {
				if resp == nil {
					t.Fatal("response is nil")
				}
				if resp.Data == nil {
					t.Fatal("response data is nil")
				}
			},
		},
		{
			name: "list roles with prefix",
			setupStorage: func(storage logical.Storage) error {
				role := &models.Role{
					Name:   "test-role",
					DBUser: "user{{username}}",
				}
				entry, err := logical.StorageEntryJSON("roles/test-role", role)
				if err != nil {
					return err
				}
				return storage.Put(context.Background(), entry)
			},
			wantErr: false,
			checkResponse: func(t *testing.T, resp *logical.Response) {
				keys, ok := resp.Data["keys"].([]string)
				if !ok {
					t.Fatal("keys not found in response")
				}
				if len(keys) != 1 || keys[0] != "test-role" {
					t.Errorf("expected keys [test-role], got %v", keys)
				}
			},
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
				Schema: map[string]*framework.FieldSchema{},
			}

			resp, err := b.pathRoleListHandler(ctx, req, data)

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

			if tt.checkResponse != nil {
				tt.checkResponse(t, resp)
			}
		})
	}
}

func TestPathRoleExistenceCheck(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		setupStorage func(logical.Storage) error
		roleName     string
		wantExists   bool
		wantErr      bool
	}{
		{
			name:       "role exists",
			roleName:   "existing",
			wantExists: true,
			setupStorage: func(storage logical.Storage) error {
				role := &models.Role{
					Name:   "existing",
					DBUser: "user{{username}}",
				}
				entry, err := logical.StorageEntryJSON("roles/existing", role)
				if err != nil {
					return err
				}
				return storage.Put(context.Background(), entry)
			},
		},
		{
			name:       "role does not exist",
			roleName:   "nonexistent",
			wantExists: false,
			setupStorage: func(storage logical.Storage) error {
				return nil
			},
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
				Raw: map[string]interface{}{
					"name": tt.roleName,
				},
				Schema: getRoleFieldSchema(),
			}

			exists, err := b.pathRoleExistenceCheck(ctx, req, data)

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

			if exists != tt.wantExists {
				t.Errorf("expected exists=%v, got %v", tt.wantExists, exists)
			}
		})
	}
}

func TestGetRole(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		setupStorage func(logical.Storage) error
		roleName     string
		wantRole     bool
		checkRole    func(*testing.T, *models.Role)
	}{
		{
			name:     "get existing role",
			roleName: "test-role",
			setupStorage: func(storage logical.Storage) error {
				role := &models.Role{
					Name:                "test-role",
					DBUser:              "user{{username}}",
					DBPassword:          "password",
					DefaultTTL:          3600,
					MaxTTL:              86400,
					CreationStatement:   "CREATE USER",
					RevocationStatement: "DROP USER",
				}
				entry, err := logical.StorageEntryJSON("roles/test-role", role)
				if err != nil {
					return err
				}
				return storage.Put(context.Background(), entry)
			},
			wantRole: true,
			checkRole: func(t *testing.T, role *models.Role) {
				if role.Name != "test-role" {
					t.Errorf("expected name 'test-role', got %s", role.Name)
				}
				if role.DBUser != "user{{username}}" {
					t.Errorf("expected db_user 'user{{username}}', got %s", role.DBUser)
				}
				if role.DefaultTTL != 3600 {
					t.Errorf("expected default_ttl 3600, got %d", role.DefaultTTL)
				}
			},
		},
		{
			name:     "get non-existent role returns nil",
			roleName: "nonexistent",
			setupStorage: func(storage logical.Storage) error {
				return nil
			},
			wantRole: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			storage := &logical.InmemStorage{}

			if err := tt.setupStorage(storage); err != nil {
				t.Fatalf("setup storage error: %v", err)
			}

			role, err := getRole(ctx, storage, tt.roleName)

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if tt.wantRole {
				if role == nil {
					t.Fatal("expected role, got nil")
				}
				if tt.checkRole != nil {
					tt.checkRole(t, role)
				}
			} else {
				if role != nil {
					t.Errorf("expected nil role, got %v", role)
				}
			}
		})
	}
}

func TestPathRoleCreateValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		roleName    string
		dbUser      string
		dbPassword  string
		defaultTTL  int
		maxTTL      int
		wantErr     bool
		errContains string
	}{
		{
			name:       "valid role with all fields",
			roleName:   "valid-role",
			dbUser:     "user{{username}}",
			dbPassword: "pass{{password}}",
			defaultTTL: 3600,
			maxTTL:     86400,
			wantErr:    false,
		},
		{
			name:        "empty role name",
			roleName:    "",
			dbUser:      "user{{username}}",
			dbPassword:  "password",
			wantErr:     false,
			errContains: "",
		},
		{
			name:       "username with valid characters",
			roleName:   "test-role",
			dbUser:     "validuser_$123",
			dbPassword: "password",
			wantErr:    false,
		},
		{
			name:       "username with special characters",
			roleName:   "test-role",
			dbUser:     "user@invalid",
			dbPassword: "password",
			wantErr:    false,
		},
		{
			name:       "empty db_user",
			roleName:   "test-role",
			dbUser:     "",
			dbPassword: "password",
			wantErr:    false,
		},
		{
			name:       "max_ttl less than default_ttl",
			roleName:   "test-role",
			dbUser:     "user{{username}}",
			dbPassword: "password",
			defaultTTL: 86400,
			maxTTL:     3600,
			wantErr:    false,
		},
		{
			name:       "zero TTL values",
			roleName:   "test-role",
			dbUser:     "user{{username}}",
			dbPassword: "password",
			defaultTTL: 0,
			maxTTL:     0,
			wantErr:    false,
		},
		{
			name:       "very long username",
			roleName:   "test-role",
			dbUser:     "this_is_a_very_long_username_that_might_exceed_teradata_limits",
			dbPassword: "password",
			wantErr:    false,
		},
		{
			name:       "username with SQL injection pattern",
			roleName:   "test-role",
			dbUser:     "user'; DROP TABLE users;--",
			dbPassword: "password",
			wantErr:    false,
		},
		{
			name:       "username with SQL keyword",
			roleName:   "test-role",
			dbUser:     "userSELECT",
			dbPassword: "password",
			wantErr:    false,
		},
		{
			name:       "username with space",
			roleName:   "test-role",
			dbUser:     "user name",
			dbPassword: "password",
			wantErr:    false,
		},
		{
			name:       "role name with spaces",
			roleName:   "role with spaces",
			dbUser:     "user{{username}}",
			dbPassword: "password",
			wantErr:    false,
		},
		{
			name:       "role name with special chars",
			roleName:   "role-with-special@chars",
			dbUser:     "user{{username}}",
			dbPassword: "password",
			wantErr:    false,
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

			data := &framework.FieldData{
				Raw: map[string]interface{}{
					"name":               tt.roleName,
					"db_user":            tt.dbUser,
					"db_password":        tt.dbPassword,
					"default_ttl":        tt.defaultTTL,
					"max_ttl":            tt.maxTTL,
					"creation_statement": "GRANT SELECT ON mydb TO {{username}}",
				},
				Schema: getRoleFieldSchema(),
			}

			resp, err := b.pathRoleCreate(ctx, req, data)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
					return
				}
				if tt.errContains != "" && !containsString(err.Error(), tt.errContains) {
					t.Errorf("expected error containing %q, got %q", tt.errContains, err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if resp != nil && resp.IsError() {
				t.Errorf("response contains error: %v", resp.Error())
			}
		})
	}
}

func TestPathRoleUpdateValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		roleName     string
		dbUser       string
		dbPassword   string
		defaultTTL   int
		maxTTL       int
		setupStorage func(logical.Storage) error
		wantErr      bool
		errContains  string
	}{
		{
			name:     "update with invalid username",
			roleName: "test-role",
			dbUser:   "invalid@user",
			setupStorage: func(storage logical.Storage) error {
				role := &models.Role{
					Name:       "test-role",
					DBUser:     "user{{username}}",
					DBPassword: "password",
				}
				entry, err := logical.StorageEntryJSON("roles/test-role", role)
				if err != nil {
					return err
				}
				return storage.Put(context.Background(), entry)
			},
			wantErr: false,
		},
		{
			name:     "update with SQL injection in username",
			roleName: "test-role",
			dbUser:   "user';DELETE--",
			setupStorage: func(storage logical.Storage) error {
				role := &models.Role{
					Name:       "test-role",
					DBUser:     "user{{username}}",
					DBPassword: "password",
				}
				entry, err := logical.StorageEntryJSON("roles/test-role", role)
				if err != nil {
					return err
				}
				return storage.Put(context.Background(), entry)
			},
			wantErr: false,
		},
		{
			name:     "update existing role valid",
			roleName: "test-role",
			dbUser:   "newuser{{username}}",
			setupStorage: func(storage logical.Storage) error {
				role := &models.Role{
					Name:       "test-role",
					DBUser:     "olduser{{username}}",
					DBPassword: "oldpass",
				}
				entry, err := logical.StorageEntryJSON("roles/test-role", role)
				if err != nil {
					return err
				}
				return storage.Put(context.Background(), entry)
			},
			wantErr: false,
		},
		{
			name:     "update with TTL mismatch",
			roleName: "test-role",
			dbUser:   "user{{username}}",
			setupStorage: func(storage logical.Storage) error {
				role := &models.Role{
					Name:       "test-role",
					DBUser:     "user{{username}}",
					DBPassword: "password",
					DefaultTTL: 100,
					MaxTTL:     50,
				}
				entry, err := logical.StorageEntryJSON("roles/test-role", role)
				if err != nil {
					return err
				}
				return storage.Put(context.Background(), entry)
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
				Raw: map[string]interface{}{
					"name":            tt.roleName,
					"db_user":         tt.dbUser,
					"db_password":     tt.dbPassword,
					"default_ttl":     tt.defaultTTL,
					"max_ttl":         tt.maxTTL,
					"max_credentials": 10,
				},
				Schema: getRoleFieldSchema(),
			}

			resp, err := b.pathRoleUpdate(ctx, req, data)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
					return
				}
				if tt.errContains != "" && !containsString(err.Error(), tt.errContains) {
					t.Errorf("expected error containing %q, got %q", tt.errContains, err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if resp != nil && resp.IsError() {
				t.Errorf("response contains error: %v", resp.Error())
			}
		})
	}
}

func containsString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if len(s) >= len(substr) && s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func getRoleFieldSchema() map[string]*framework.FieldSchema {
	return map[string]*framework.FieldSchema{
		"name": {
			Type: framework.TypeString,
		},
		"db_user": {
			Type: framework.TypeString,
		},
		"db_password": {
			Type: framework.TypeString,
		},
		"default_ttl": {
			Type: framework.TypeInt,
		},
		"max_ttl": {
			Type: framework.TypeInt,
		},
		"renewal_period": {
			Type: framework.TypeInt,
		},
		"statement_template": {
			Type: framework.TypeString,
		},
		"default_database": {
			Type: framework.TypeString,
		},
		"perm_space": {
			Type: framework.TypeInt,
		},
		"spool_space": {
			Type: framework.TypeInt,
		},
		"account": {
			Type: framework.TypeString,
		},
		"fallback": {
			Type: framework.TypeBool,
		},
		"username_prefix": {
			Type: framework.TypeString,
		},
		"username_suffix": {
			Type: framework.TypeString,
		},
		"creation_statement": {
			Type: framework.TypeString,
		},
		"revocation_statement": {
			Type: framework.TypeString,
		},
		"rollback_statement": {
			Type: framework.TypeString,
		},
		"renewal_statement": {
			Type: framework.TypeString,
		},
		"max_credentials": {
			Type: framework.TypeInt,
		},
	}
}
