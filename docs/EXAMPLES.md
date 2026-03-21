# Teradata Secret Plugin - Examples

Usage examples and tutorials.

---

## Quick Start

### 1. Register the Plugin

```bash
# Get SHA256 hash
SHA256=$(shasum -a 256 bin/teradata-plugin | cut -d' ' -f1)

# Register in OpenBAO
bao plugin register \
    -path=teradata \
    -command=teradata-plugin \
    -sha256=$SHA256 \
    teradata-plugin

# Enable the plugin
bao secrets enable -path=teradata teradata-plugin
```

### 2. Configure Connection

```bash
# Using DSN
bao write teradata/config \
    connection_string="DSN=MyTeradata;UID=admin;PWD=password"

# Or direct connection
bao write teradata/config \
    connection_string="DRIVER={Teradata};DBCNAME=hostname;UID=admin;PWD=password"
```

---

## Role Examples

### Example 1: Read-Only Role

```bash
# Create role with SELECT only
bao write teradata/roles/readonly \
    db_user="vault_readonly_{{uuid}}" \
    default_ttl=3600 \
    max_ttl=86400 \
    default_database=mydb \
    creation_statement="GRANT SELECT ON mydb TO {{username}};"

# Generate credentials
bao read teradata/creds/readonly

# Output:
# password: a1b2c3d4e5f6...
# ttl: 3600
# username: vault_readonly_abc123
```

### Example 2: Read-Write Role

```bash
# Create role with full access
bao write teradata/roles/readwrite \
    db_user="vault_readwrite_{{uuid}}" \
    default_ttl=3600 \
    max_ttl=86400 \
    default_database=mydb \
    creation_statement="
        GRANT SELECT ON mydb TO {{username}};
        GRANT INSERT ON mydb TO {{username}};
        GRANT UPDATE ON mydb TO {{username}};
        GRANT DELETE ON mydb TO {{username}};
    "
```

### Example 3: Role with Space Limits

```bash
# Create role with PERM and SPOOL limits
bao write teradata/roles/app \
    db_user="vault_app_{{uuid}}" \
    default_ttl=7200 \
    max_ttl=14400 \
    default_database=appdb \
    perm_space=1000000000 \
    spool_space=2000000000 \
    fallback=true \
    creation_statement="GRANT ALL ON appdb TO {{username}};"
```

---

## SQL Statement Templates

### Available Placeholders

| Placeholder | Description |
|------------|-------------|
| `{{username}}` | Generated username |
| `{{password}}` | Generated password |
| `{{uuid}}` | Random UUID (in username) |

### Common GRANT Statements

```sql
-- Single database
GRANT SELECT ON mydb TO {{username}};
GRANT SELECT, INSERT, UPDATE, DELETE ON mydb TO {{username}};
GRANT ALL ON mydb TO {{username}};

-- Multiple databases
GRANT SELECT ON db1 TO {{username}};
GRANT SELECT ON db2 TO {{username}};

-- Execute stored procedures
GRANT EXECUTE ON myproc TO {{username}};
```

### Revocation Statements

```sql
-- Revoke all before dropping
REVOKE ALL ON mydb FROM {{username}};
```

---

## Using Credentials

### Directly in Application

```python
import teradatasql

# Get credentials from OpenBAO
creds = bao.read("teradata/creds/readonly")

# Connect to Teradata
conn = teradatasql.connect(
    host="hostname",
    user=creds["username"],
    password=creds["password"]
)
```

### Via Environment

```bash
# Get credentials
export TERADATA_USER=$(bao read -field=username teradata/creds/readonly)
export TERADATA_PASS=$(bao read -field=password teradata/creds/readonly)

# Use in application
python app.py
```

---

## Configuration Options

### Connection String Parameters

| Parameter | Description | Example |
|-----------|-------------|---------|
| DBCNAME | Teradata server hostname | `myhost.example.com` |
| UID | Username | `admin` |
| PWD | Password | `password` |
| DSN | Data Source Name | `MyTeradata` |
| DRIVER | ODBC driver path | `/path/to/driver` |

### Full Example

```bash
bao write teradata/config \
    connection_string="DRIVER={Teradata};DBCNAME=teradata.example.com;UID=admin;PWD=secret;LOG=1" \
    max_open_connections=10 \
    max_idle_connections=5 \
    connection_timeout=30
```

---

## Testing

### Verify Configuration

```bash
# Check health
bao read teradata/health

# Check version
bao read teradata/version

# List roles
bao list teradata/roles
```

### Test Connection

```bash
# Read config (password hidden)
bao read teradata/config
```
