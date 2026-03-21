# Core Tasks - Teradata Secret Plugin

Core functionality implementation.

## Status
- Total: 8
- Completed: 0
- Pending: 8

---

## T-007: Implement ODBC Connection Pool

**Priority**: HIGH | **Status**: pending

Need proper ODBC connection management.

### Sub-tasks
- [ ] T-007.1: Implement connection pooling in `storage/db_registry.go`
- [ ] T-007.2: Add connection health checks
- [ ] T-007.3: Handle connection timeouts

### Dependencies
- T-003 (Implement Backend)

### Implementation
```go
type DBConnection struct {
    Config   *DBConfig
    Database *sql.DB
    mu       sync.RWMutex
}
```

---

## T-008: Implement Credential Generation

**Priority**: HIGH | **Status**: pending

Generate dynamic database credentials.

### Sub-tasks
- [ ] T-008.1: Implement password generation (secure random)
- [ ] T-008.2: Generate unique usernames
- [ ] T-008.3: Handle credential storage

### Dependencies
- T-005 (Role Management)

---

## T-009: Implement CREATE USER SQL

**Priority**: HIGH | **Status**: pending

Execute CREATE USER on Teradata.

### Sub-tasks
- [ ] T-009.1: Build CREATE USER SQL statement
- [ ] T-009.2: Handle PERM space requirements
- [ ] T-009.3: Handle DEFAULT DATABASE

### Teradata Syntax
```sql
CREATE USER username FROM DBC AS
    PASSWORD = password
    DEFAULT DATABASE = dbname
    PERM = 1000000;
```

---

## T-010: Implement GRANT Statements

**Priority**: HIGH | **Status**: pending

Execute GRANT after user creation.

### Sub-tasks
- [ ] T-010.1: Parse creation_statement for GRANTs
- [ ] T-010.2: Handle multiple GRANT statements
- [ ] T-010.3: Handle errors gracefully

### Dependencies
- T-009 (CREATE USER)

---

## T-011: Implement REVOKE Statements

**Priority**: HIGH | **Status**: pending

Revoke before dropping user.

### Sub-tasks
- [ ] T-011.1: Parse revocation_statement
- [ ] T-011.2: Execute REVOKE before DROP USER
- [ ] T-011.3: Handle missing privileges

---

## T-012: Implement DROP USER

**Priority**: HIGH | **Status**: pending

Clean up user on credential revocation.

### Sub-tasks
- [ ] T-012.1: Execute DROP USER SQL
- [ ] T-012.2: Handle cascade delete
- [ ] T-012.3: Handle errors

---

## T-013: Implement Connection Validation

**Priority**: MEDIUM | **Status**: pending

Validate ODBC connection works.

### Sub-tasks
- [ ] T-013.1: Test connection on config write
- [ ] T-013.2: Add health check endpoint
- [ ] T-013.3: Handle connection failures

---

## T-014: Statement Templates

**Priority**: MEDIUM | **Status**: pending

Support reusable SQL statements.

### Sub-tasks
- [ ] T-014.1: Create statement storage
- [ ] T-014.2: Implement statement CRUD
- [ ] T-014.3: Link statements to roles

---

## Blocked Issues

### Issue 1: ODBC Connection Not Implemented
**Status**: pending | **Priority**: HIGH

**Location**: `odbc/odbc.go`

**Description**: The ODBC connection is currently a placeholder. Need to implement actual cgo-based ODBC connection.

**Solution**: 
1. Use cgo to call ODBC driver
2. Implement sql.DB interface
3. Handle connection pooling

### Issue 2: CREATE USER Needs Admin Permission
**Status**: pending | **Priority**: HIGH

**Description**: Current demo user doesn't have permission to CREATE USER in DBC.

**Solution**: 
1. Use admin credentials for connection
2. Or create users in a specific database

---

## Verification Commands

```bash
# Test ODBC connection
go test -v ./odbc/ -run TestConnection

# Test credential generation
go test -v ./plugin/ -run TestCreds

# Expected: All tests pass
```
