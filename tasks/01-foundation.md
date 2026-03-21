# Foundation Tasks - Teradata Secret Plugin

Tasks required to set up the basic plugin structure.

## Status
- Total: 6
- Completed: 1
- Pending: 5

---

## T-001: Initialize Project Structure

**Priority**: HIGH | **Status**: completed

Go module already initialized.

### Sub-tasks
- [x] T-001.1: Initialize `go.mod` with module name
- [x] T-001.2: Add dependencies: OpenBao SDK
- [x] T-001.3: Create `go.sum` via `go mod tidy`
- [x] T-001.4: Set up project directory structure

---

## T-002: Create Plugin Entry Point

**Priority**: HIGH | **Status**: completed

main.go already created.

### Sub-tasks
- [x] T-002.1: Create `cmd/teradata/main.go`
- [x] T-002.2: Implement `plugin.ServeMultiplex` with `BackendFactoryFunc`
- [x] T-002.3: Add TLS provider configuration

---

## T-003: Implement Backend

**Priority**: HIGH | **Status**: completed

Backend implementation already exists.

### Sub-tasks
- [x] T-003.1: Define `Backend` struct in `plugin/backend.go`
- [x] T-003.2: Implement `Factory` function
- [x] T-003.3: Register paths with OpenBao framework

---

## T-004: Configuration Path

**Priority**: HIGH | **Status**: completed

Configuration CRUD already implemented.

### Sub-tasks
- [x] T-004.1: Define Configuration model in `models/config.go`
- [x] T-004.2: Implement `pathConfig` with read/write/delete
- [x] T-004.3: Add config validation and defaults

---

## T-005: Role Management Path

**Priority**: HIGH | **Status**: completed

Role management already implemented.

### Sub-tasks
- [x] T-005.1: Define Role model in `models/config.go`
- [x] T-005.2: Implement roles list path
- [x] T-005.3: Implement role CRUD paths

---

## T-006: Basic Project Files

**Priority**: MEDIUM | **Status**: completed

Project files already created.

### Sub-tasks
- [x] T-006.1: Create Makefile with build, test targets
- [x] T-006.2: Create .gitignore
- [x] T-006.3: Create README.md

---

## Implementation Notes

### ODBC Connection String Format

```
DRIVER={Teradata Database ODBC Driver 20.00};DBCNAME=hostname;UID=user;PWD=password
```

Or with DSN:
```
DSN=MyTeradata;UID=user;PWD=password
```

### Teradata SQL Syntax

```sql
CREATE USER username FROM DBC AS
    PASSWORD = password
    DEFAULT DATABASE = dbname
    PERM = 1000000;

GRANT SELECT ON database TO username;

DROP USER username;
```

### Key Differences from Other Databases

1. PERM is required in Teradata Cloud
2. No quotes around password
3. FROM DBC is required
4. AS keyword is required

---

## Reference Implementations

| Plugin | Maturity | What to Copy |
|--------|----------|--------------|
| transform | 100% | All features, patterns, tests |
| kmip | 70% | Security patterns, audit |
| teradata | 10% | Foundation done, needs testing |
