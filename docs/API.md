# Teradata Secret Plugin - API Reference

This document provides a complete API reference for the Teradata Secret Plugin.

## Table of Contents

- [Base Path](#base-path)
- [Authentication](#authentication)
- [Configuration](#configuration)
- [Roles](#roles)
- [Credentials](#credentials)
- [Revoke Credentials](#revoke-credentials)
- [Renew Credentials](#renew-credentials)
- [Rotate Root](#rotate-root)
- [System Endpoints](#system-endpoints)
- [Error Responses](#error-responses)

---

## Base Path

All Teradata plugin endpoints are prefixed with the mount path (default: `teradata`):

```
/teradata/
```

---

## Authentication

All requests require a valid OpenBao token with appropriate policies.

---

## Configuration

### Create/Update Configuration

| Attribute | Value |
|-----------|-------|
| **Endpoint** | `POST /teradata/config` |
| **Description** | Configures the Teradata database connection |

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `connection_string` | string | yes | ODBC connection string |
| `max_open_connections` | int | no | Max open connections (default: 5) |
| `max_idle_connections` | int | no | Max idle connections (default: 2) |
| `connection_timeout` | int | no | Timeout in seconds (default: 30) |

**Example Request:**
```json
{
  "connection_string": "DSN=MyTeradata;UID=admin;PWD=password",
  "max_open_connections": 5,
  "max_idle_connections": 2
}
```

**Example Response:**
```json
{
  "data": {
    "connection_string": "***",
    "max_open_connections": 5,
    "max_idle_connections": 2,
    "connection_timeout": 30
  }
}
```

**curl Example:**
```bash
curl -X POST http://127.0.0.1:8200/v1/teradata/config \
  -H "X-Vault-Token: your-vault-token" \
  -d '{
    "connection_string": "DSN=MyTeradata;UID=admin;PWD=password",
    "max_open_connections": 5,
    "max_idle_connections": 2,
    "connection_timeout": 30
  }'
```

---

### Read Configuration

| Attribute | Value |
|-----------|-------|
| **Endpoint** | `GET /teradata/config` |
| **Description** | Retrieves the current configuration |

**curl Example:**
```bash
curl -X GET http://127.0.0.1:8200/v1/teradata/config \
  -H "X-Vault-Token: your-vault-token"
```

---

### Delete Configuration

| Attribute | Value |
|-----------|-------|
| **Endpoint** | `DELETE /teradata/config` |
| **Description** | Removes the configuration |

**curl Example:**
```bash
curl -X DELETE http://127.0.0.1:8200/v1/teradata/config \
  -H "X-Vault-Token: your-vault-token"
```

---

## Roles

### Create Role

| Attribute | Value |
|-----------|-------|
| **Endpoint** | `POST /teradata/roles/:name` |
| **Description** | Creates a new role |

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `name` | path | yes | Role name |
| `db_user` | string | yes | Database username template |
| `default_ttl` | int | no | Default lease TTL in seconds (default: 3600) |
| `max_ttl` | int | no | Maximum lease TTL in seconds (default: 86400) |
| `default_database` | string | no | Default database (default: USER) |
| `perm_space` | int | no | Permanent space in bytes (0 = unlimited) |
| `spool_space` | int | no | Spool space in bytes |
| `account` | string | no | Account string |
| `fallback` | bool | no | Enable fallback protection (default: false) |
| `creation_statement` | string | no | Additional SQL after CREATE USER |
| `revocation_statement` | string | no | SQL to run before DROP USER |

**Example Request:**
```json
{
  "db_user": "vault_user",
  "default_ttl": 3600,
  "max_ttl": 86400,
  "default_database": "mydb",
  "perm_space": 1000000000,
  "spool_space": 500000000,
  "fallback": true,
  "creation_statement": "GRANT SELECT ON mydb TO {{username}};"
}
```

**curl Example:**
```bash
curl -X POST http://127.0.0.1:8200/v1/teradata/roles/readonly \
  -H "X-Vault-Token: your-vault-token" \
  -d '{
    "db_user": "vault_user",
    "default_ttl": 3600,
    "max_ttl": 86400,
    "default_database": "mydb",
    "perm_space": 1000000000,
    "spool_space": 500000000,
    "fallback": true,
    "creation_statement": "GRANT SELECT ON mydb TO {{username}};"
  }'
```

---

### Read Role

| Attribute | Value |
|-----------|-------|
| **Endpoint** | `GET /teradata/roles/:name` |
| **Description** | Retrieves role configuration |

**curl Example:**
```bash
curl -X GET http://127.0.0.1:8200/v1/teradata/roles/readonly \
  -H "X-Vault-Token: your-vault-token"
```

---

### List Roles

| Attribute | Value |
|-----------|-------|
| **Endpoint** | `LIST /teradata/roles` |
| **Description** | Lists all configured roles |

**Example Response:**
```json
{
  "data": {
    "keys": ["admin", "readonly", "readwrite"]
  }
}
```

**curl Example:**
```bash
curl -X LIST http://127.0.0.1:8200/v1/teradata/roles \
  -H "X-Vault-Token: your-vault-token"
```

---

### Delete Role

| Attribute | Value |
|-----------|-------|
| **Endpoint** | `DELETE /teradata/roles/:name` |
| **Description** | Deletes the specified role |

**curl Example:**
```bash
curl -X DELETE http://127.0.0.1:8200/v1/teradata/roles/readonly \
  -H "X-Vault-Token: your-vault-token"
```

---

## Credentials

### Generate Credentials

| Attribute | Value |
|-----------|-------|
| **Endpoint** | `GET /teradata/creds/:name` |
| **Description** | Generates dynamic database credentials |

**Example Response:**
```json
{
  "data": {
    "username": "vault_user_a1b2c3d4",
    "password": "SecureP@ss123!",
    "ttl": 3600,
    "max_ttl": 86400
  }
}
```

**curl Example:**
```bash
curl -X GET http://127.0.0.1:8200/v1/teradata/creds/readonly \
  -H "X-Vault-Token: your-vault-token"
```

---

## Revoke Credentials

### Revoke Credentials

| Attribute | Value |
|-----------|-------|
| **Endpoint** | `DELETE /teradata/revoke-cred/:username` |
| **Description** | Revokes and deletes dynamically generated database credentials |

**Example Response:**
```json
{
  "data": {
    "revoked": true
  }
}
```

**curl Example:**
```bash
curl -X DELETE http://127.0.0.1:8200/v1/teradata/revoke-cred/vault_user_a1b2c3d4 \
  -H "X-Vault-Token: your-vault-token"
```

---

## Renew Credentials

### Renew Credentials

| Attribute | Value |
|-----------|-------|
| **Endpoint** | `POST /teradata/renew-cred/:username` |
| **Description** | Rotates the password for dynamically generated database credentials |

**Example Response:**
```json
{
  "data": {
    "username": "vault_user_a1b2c3d4",
    "password": "NewSecureP@ss123!",
    "ttl": 3600,
    "max_ttl": 86400,
    "last_renewed": 1699999999,
    "expires_at": 1700003599
  }
}
```

**curl Example:**
```bash
curl -X POST http://127.0.0.1:8200/v1/teradata/renew-cred/vault_user_a1b2c3d4 \
  -H "X-Vault-Token: your-vault-token"
```

---

## Rotate Root

### Rotate Root Credentials

| Attribute | Value |
|-----------|-------|
| **Endpoint** | `POST /teradata/rotate-root` |
| **Description** | Rotates the root database credentials |

**Example Response:**
```json
{
  "data": {
    "rotated": true
  }
}
```

**curl Example:**
```bash
curl -X POST http://127.0.0.1:8200/v1/teradata/rotate-root \
  -H "X-Vault-Token: your-vault-token"
```

---

## System Endpoints

### Health

| Attribute | Value |
|-----------|-------|
| **Endpoint** | `GET /teradata/health` |
| **Description** | Returns plugin health status |

**Example Response:**
```json
{
  "data": {
    "status": "healthy",
    "initialized": true
  }
}
```

**curl Example:**
```bash
curl -X GET http://127.0.0.1:8200/v1/teradata/health \
  -H "X-Vault-Token: your-vault-token"
```

---

### Version

| Attribute | Value |
|-----------|-------|
| **Endpoint** | `GET /teradata/version` |
| **Description** | Returns plugin version |

**Example Response:**
```json
{
  "data": {
    "version": "0.1.0"
  }
}
```

**curl Example:**
```bash
curl -X GET http://127.0.0.1:8200/v1/teradata/version \
  -H "X-Vault-Token: your-vault-token"
```

---

## Error Responses

| Status Code | Description |
|-------------|-------------|
| `400 Bad Request` | Invalid request parameters |
| `403 Forbidden` | Permission denied |
| `404 Not Found` | Resource not found |
| `500 Internal Server Error` | Server error |

**Example Error:**
```json
{
  "errors": [
    "role not found"
  ]
}
```

---

## Connection String Format

The connection string follows standard ODBC format:

```
DSN=<data_source_name>;UID=<username>;PWD=<password>;
```

Or without DSN:

```
DRIVER={Teradata};DBCNAME=<database>;
UID=<username>;PWD=<password>;
```

---

## SQL Statement Templates

Use these placeholders in your SQL statements:

| Placeholder | Description |
|-------------|-------------|
| `{{username}}` | Generated username |
| `{{password}}` | Generated password |

### Teradata CREATE USER Syntax

The plugin automatically generates the CREATE USER statement based on role settings:

```sql
CREATE USER vault_user FROM DBC AS
PASSWORD = 'generated_password'
DEFAULT DATABASE = mydb
PERM = 1000000000
SPOOL = 500000000
FALLBACK
;
```

### Additional SQL (creation_statement)

After user creation, you can run additional SQL:

```sql
-- Grant privileges
GRANT SELECT ON mydb TO {{username}};
GRANT INSERT ON mydb TO {{username}};

-- Set up row-level security
GRANT SELECT ON sensitive_data TO {{username}};
```

### Revocation (revocation_statement)

Before dropping the user:

```sql
-- Revoke all privileges
REVOKE ALL ON mydb FROM {{username}};
```
