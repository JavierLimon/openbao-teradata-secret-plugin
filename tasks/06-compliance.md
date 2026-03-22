# OpenBAO Database Plugin Compliance Tasks

Make the Teradata plugin compliant with the standard OpenBAO database plugin API.

## Status
- Total: 12
- Completed: 0
- Pending: 12

---

## Background

### Standard OpenBAO Database Plugin API
The plugin should implement the standard database secrets engine endpoints as documented at:
https://developer.hashicorp.com/vault/api-docs/secret/databases#database-secrets-engine-api

### Current Issues
1. Config path uses `region` instead of `name` - `/teradata/config/:region` vs `/database/config/:name`
2. Missing standard config fields: `plugin_name`, `plugin_version`, `allowed_roles`, `root_rotation_statements`, `password_policy`
3. Missing LIST /config endpoint
4. Missing reset/reload endpoints
5. Missing static roles endpoints

---

## T-034: Standardize Config Endpoint

**Priority**: HIGH | **Status**: pending

Update config to match standard database plugin API.

### Sub-tasks
- [ ] T-034.1: Change path from `config/(?P<region>[a-zA-Z0-9_-]+)` to `config/(?P<name>[a-zA-Z0-9_-]+)`
- [ ] T-034.2: Add field: `plugin_name` (string, required)
- [ ] T-034.3: Add field: `plugin_version` (string, default empty)
- [ ] T-034.4: Add field: `verify_connection` (bool, default true)
- [ ] T-034.5: Add field: `allowed_roles` (list, default empty)
- [ ] T-034.6: Add field: `root_rotation_statements` (list)
- [ ] T-034.7: Add field: `password_policy` (string, default empty)
- [ ] T-034.8: Add field: `connection_url` (string) - replaces connection_string for compatibility
- [ ] T-034.9: Add field: `username` (string) - root user for connections
- [ ] T-034.10: Add field: `password` (string) - root password (write-only)
- [ ] T-034.11: Add field: `disable_escaping` (bool, default false)
- [ ] T-034.12: Update response to mask password and show connection_details

### Implementation
```go
// New config fields to add
type Config struct {
    Name                       string   `json:"name"`
    PluginName                 string   `json:"plugin_name"`
    PluginVersion              string   `json:"plugin_version"`
    ConnectionURL              string   `json:"connection_url"`
    Username                   string   `json:"username"`
    AllowedRoles               []string `json:"allowed_roles"`
    RootRotationStatements    []string `json:"root_rotation_statements"`
    PasswordPolicy             string   `json:"password_policy"`
    VerifyConnection          bool     `json:"verify_connection"`
    DisableEscaping            bool     `json:"disable_escaping"`
    // Keep existing fields for backward compatibility
    ConnectionString          string   `json:"connection_string,omitempty"`
    MaxOpenConnections         int      `json:"max_open_connections"`
    MaxIdleConnections         int      `json:"max_idle_connections"`
    ConnectionTimeout         int      `json:"connection_timeout"`
}
```

### Deprecation Strategy
- Support both `connection_string` and `connection_url` temporarily
- Log deprecation warning when `connection_string` is used
- Remove `connection_string` in next major version

---

## T-035: Add List Config Endpoint

**Priority**: HIGH | **Status**: pending

Add `LIST /database/config` endpoint.

### Sub-tasks
- [ ] T-035.1: Create path pattern `config?$` or `config/$`
- [ ] T-035.2: Implement list operation returning all config keys
- [ ] T-035.3: Return only names, not sensitive data

### Implementation
```go
func (b *Backend) pathConfigList() *framework.Path {
    return &framework.Path{
        Pattern: "config/?$",
        Operations: map[logical.Operation]framework.OperationHandler{
            logical.ListOperation: &framework.PathOperation{
                Callback: b.pathConfigListHandler,
            },
        },
    }
}
```

---

## T-036: Add Reset Connection Endpoint

**Priority**: MEDIUM | **Status**: pending

Add `POST /database/reset/:name` endpoint.

### Sub-tasks
- [ ] T-036.1: Create path pattern `reset/(?P<name>[a-zA-Z0-9_-]+)`
- [ ] T-036.2: Close existing connection in registry
- [ ] T-036.3: Restart plugin with new config
- [ ] T-036.4: Verify connection after reset
- [ ] T-036.5: Return success/failure status

### Implementation
```go
func (b *Backend) pathReset() *framework.Path {
    return &framework.Path{
        Pattern: "reset/(?P<name>[a-zA-Z0-9_-]+)",
        Operations: map[logical.Operation]framework.OperationHandler{
            logical.CreateOperation: &framework.PathOperation{
                Callback: b.pathResetHandler,
            },
        },
    }
}
```

---

## T-037: Add Reload Plugin Endpoint

**Priority**: MEDIUM | **Status**: pending

Add `POST /database/reload/:plugin_name` endpoint.

### Sub-tasks
- [ ] T-037.1: Create path pattern `reload/(?P<plugin_name>[a-zA-Z0-9_-]+)`
- [ ] T-037.2: Find all connections using this plugin
- [ ] T-037.3: Reset each connection
- [ ] T-037.4: Return count of reloaded connections

---

## T-038: Implement Static Roles

**Priority**: HIGH | **Status**: pending

Add static role support for long-lived database credentials.

### Sub-tasks
- [ ] T-038.1: Add static role model with fields:
  - `username` (required) - database username
  - `password` (string) - current password
  - `db_name` (string) - connection name
  - `rotation_period` (int) - seconds between rotations
  - `rotation_schedule` (string) - cron-style schedule
  - `rotation_window` (int) - allowed window for rotation
  - `rotation_statements` (list)
  - `credential_type` (string, default "password")
  - `credential_config` (map)
- [ ] T-038.2: Implement `POST /static-roles/:name`
- [ ] T-038.3: Implement `GET /static-roles/:name`
- [ ] T-038.4: Implement `LIST /static-roles`
- [ ] T-038.5: Implement `DELETE /static-roles/:name`
- [ ] T-038.6: Implement `GET /static-creds/:name`
- [ ] T-038.7: Implement `POST /rotate-role/:name`

### Static Role Model
```go
type StaticRole struct {
    Username              string            `json:"username"`
    DBName               string            `json:"db_name"`
    RotationPeriod       int               `json:"rotation_period"`
    RotationSchedule     string            `json:"rotation_schedule"`
    RotationWindow       int               `json:"rotation_window"`
    RotationStatements   []string          `json:"rotation_statements"`
    CredentialType       string            `json:"credential_type"`
    CredentialConfig    map[string]string `json:"credential_config"`
    LastVaultRotation   time.Time         `json:"last_vault_rotation"`
    RotationProgress    bool              `json:"rotation_progress"`
}
```

### Credential Response Format
```json
{
  "username": "static-user",
  "password": "132ae3ef-5a64-7499-351e-bfe59f3a2a21",
  "last_vault_rotation": "2019-05-06T15:26:42.525302-05:00",
  "rotation_period": 30,
  "ttl": 28
}
```

---

## T-039: Add Role-Based Access Control

**Priority**: MEDIUM | **Status**: pending

Implement allowed_roles checking in credential generation.

### Sub-tasks
- [ ] T-039.1: Check config.allowed_roles during credential generation
- [ ] T-039.2: Allow if list contains "*" or role name
- [ ] T-039.3: Return error if role not allowed
- [ ] T-039.4: Add tests for role restriction

---

## T-040: Add Connection Verification

**Priority**: MEDIUM | **Status**: pending

Verify connection during config write when verify_connection=true.

### Sub-tasks
- [ ] T-040.1: Test connection after config save
- [ ] T-040.2: Rollback config if verification fails
- [ ] T-040.3: Add timeout for verification
- [ ] T-040.4: Return clear error message on failure

---

## T-041: Update Role Model for Standard API

**Priority**: MEDIUM | **Status**: pending

Update role model to match standard database plugin.

### Sub-tasks
- [ ] T-041.1: Rename `db_name` field to match (already correct)
- [ ] T-041.2: Add `credential_type` field (password, rsa_private_key, client_certificate)
- [ ] T-041.3: Add `credential_config` map field
- [ ] T-041.4: Support password policy in credential_config

### Role Fields
```go
type Role struct {
    // Existing fields
    Name                  string   `json:"name"`
    DBName                string   `json:"db_name"`
    DefaultTTL            int      `json:"default_ttl"`
    MaxTTL                int      `json:"max_ttl"`
    CreationStatement    string   `json:"creation_statement"`
    RevocationStatement  string   `json:"revocation_statement"`
    RollbackStatement    string   `json:"rollback_statement"`
    RenewStatement       string   `json:"renew_statement"`
    
    // New fields
    CredentialType       string            `json:"credential_type"`
    CredentialConfig     map[string]string `json:"credential_config"`
}
```

---

## T-042: Implement Password Policy Support

**Priority**: LOW | **Status**: pending

Support password policies for credential generation.

### Sub-tasks
- [ ] T-042.1: Accept password_policy in role and config
- [ ] T-042.2: Generate passwords using policy
- [ ] T-042.3: Use default policy if not specified
- [ ] T-042.4: Support Vault password policies

---

## T-043: Deprecation & Migration

**Priority**: LOW | **Status**: pending

Handle backward compatibility and migrations.

### Sub-tasks
- [ ] T-043.1: Add migration function for old config format
- [ ] T-043.2: Log deprecation warnings
- [ ] T-043.3: Document migration path
- [ ] T-043.4: Add tests for migration

---

## T-044: Update Tests

**Priority**: HIGH | **Status**: pending

Update tests for compliance changes.

### Sub-tasks
- [ ] T-044.1: Update config tests for new fields
- [ ] T-044.2: Add tests for list config
- [ ] T-044.3: Add tests for static roles
- [ ] T-044.4: Add tests for role allowed list
- [ ] T-044.5: Add integration tests for static role rotation

---

## T-045: Update Documentation

**Priority**: MEDIUM | **Status**: pending

Update API documentation.

### Sub-tasks
- [ ] T-045.1: Update API.md with new endpoints
- [ ] T-045.2: Document static roles
- [ ] T-045.3: Document password policies
- [ ] T-045.4: Add migration guide

---

## Verification Commands

```bash
# Test new endpoints
go test -v ./plugin/ -run "TestConfig"

# Test static roles
go test -v ./plugin/ -run "TestStaticRole"

# Run all tests
go test -v ./...

# Test manually with Vault
vault write teradata/config/my-connection \
    plugin_name="teradata-database-plugin" \
    connection_url="{{username}}:{{password}}@tcp(localhost:1025)/" \
    username="admin" \
    password="password" \
    allowed_roles="*"

vault list teradata/config

vault write teradata/static-roles/my-static \
    db_name="my-connection" \
    username="db_user" \
    rotation_period="3600"

vault read teradata/static-creds/my-static
```

---

## Reference

- [OpenBAO Database Plugin API](https://developer.hashicorp.com/vault/api-docs/secret/databases)
- [Vault Database Secrets Engine](https://developer.hashicorp.com/vault/docs/secrets/databases)
- [Sample Database Plugins](https://github.com/hashicorp/vault/tree/main/plugins/database)
