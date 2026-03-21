# Teradata Secret Plugin - Troubleshooting

Common issues and solutions.

---

## Connection Issues

### Error: "Can't open lib 'Teradata Database ODBC Driver'"

**Problem**: ODBC driver not found

**Solution**:
```bash
# Verify driver is installed
odbcinst -q -d

# If not found, install:
# Download from https://downloads.teradata.com/
sudo installer -pkg TeradataODBC*.pkg -target /
```

---

### Error: "Operation not allowed during the transaction state"

**Problem**: Need autocommit enabled

**Solution**:
```python
# Add autocommit=True to connection
conn = pyodbc.connect(conn_str, autocommit=True)
```

---

### Error: "The user does not have CREATE USER access"

**Problem**: Insufficient permissions

**Solution**:
1. Use admin credentials for config
2. Or grant CREATE USER permission:
```sql
GRANT CREATE USER TO admin_user;
```

---

## Credential Issues

### Error: "Syntax error, expected something like..."

**Problem**: Incorrect SQL syntax

**Solution**:
```bash
# Check CREATE USER syntax - no quotes around password
# Correct:
CREATE USER name FROM DBC AS PASSWORD = password

# Wrong:
CREATE USER name FROM DBC AS PASSWORD = 'password'
```

---

### Error: "The user must specify a value for PERMANENT space"

**Problem**: PERM required in Teradata Cloud

**Solution**:
```bash
# Add PERM to role
bao write teradata/roles/myrole \
    db_user="vault_{{uuid}}" \
    perm_space=1000000
```

---

## Build Issues

### Error: "go: invalid flag in #cgo CFLAGS"

**Problem**: Path with spaces not quoted

**Solution**:
```go
// Use quoted path
#cgo CFLAGS: -I"/Library/Application Support/teradata/client/20.00/include"
```

---

### Error: "module not found"

**Problem**: Go module issues

**Solution**:
```bash
go mod tidy
go mod download
```

---

## Testing Issues

### Error: "connection refused"

**Problem**: Teradata not accessible

**Solution**:
1. Check network connectivity
2. Verify firewall rules
3. Check Teradata is running

---

## Debugging

### Enable ODBC Logging

```bash
# Add to connection string
LOG=1;LOGDIR=/tmp;
```

### Check Plugin Logs

```bash
# Run with verbose logging
VAULT_LOG_LEVEL=debug bao server
```

### Test Connection Manually

```python
import pyodbc
conn = pyodbc.connect("DSN=YourDSN;UID=user;PWD=pass")
cursor = conn.cursor()
cursor.execute("SELECT 1")
print(cursor.fetchone())
```

---

## Common Error Codes

| Code | Description | Solution |
|------|-------------|-----------|
| -3707 | Syntax error | Check SQL syntax |
| -3524 | No CREATE USER permission | Use admin user |
| -3796 | PERM required | Add PERM to role |
| 11210 | Transaction state | Enable autocommit |

---

## Getting Help

1. Check [API.md](./API.md)
2. Check [EXAMPLES.md](./EXAMPLES.md)
3. Run with debug logging
4. Check OpenBao logs
