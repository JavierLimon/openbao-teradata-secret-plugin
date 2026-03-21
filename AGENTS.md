# OpenBAO Teradata Secret Plugin - AI Agent Tasks

> **Quick Index** - See detailed tasks in separate files below

## Status Legend
| Status | Meaning |
|--------|---------|
| `pending` | Not started |
| `in_progress` | Currently working |
| `completed` | Done and verified |
| `blocked` | Waiting on dependency |

---

## Quick Status Summary

| Category | Total | Completed | Pending | In Progress |
|----------|-------|----------|---------|-------------|
| **Foundation** | 6 | 1 | 5 | 0 |
| **Core** | 8 | 0 | 8 | 0 |
| **API** | 5 | 0 | 5 | 0 |
| **Security** | 6 | 0 | 6 | 0 |
| **Tests** | 8 | 0 | 8 | 0 |
| **TOTAL** | **33** | **1** | **32** | **0** |

---

## Task Categories

| # | Category | File | Status |
|---|----------|------|--------|
| 01 | Foundation | [tasks/01-foundation.md](./tasks/01-foundation.md) | in_progress |
| 02 | Core | [tasks/02-core.md](./tasks/02-core.md) | pending |
| 03 | API | [tasks/03-api.md](./tasks/03-api.md) | pending |
| 04 | Security | [tasks/04-security.md](./tasks/04-security.md) | pending |
| 05 | Testing | [tasks/05-testing.md](./tasks/05-testing.md) | pending |

---

## File Structure

```
AGENTS.md              <- This file (INDEX)
├── tasks/
│   ├── 01-foundation.md   <- Project setup
│   ├── 02-core.md         <- Core functionality
│   ├── 03-api.md          <- API endpoints
│   ├── 04-security.md     <- Security tests
│   └── 05-testing.md     <- Unit tests
└── docs/                   <- Project documentation
```

---

## Project Overview

- **Project Name**: openbao-teradata-secret-plugin
- **Type**: Database Secret Engine Plugin for OpenBao
- **Core Functionality**: Dynamic database credentials for Teradata using ODBC
- **Module Path**: `github.com/JavierLimon/openbao-teradata-secret-plugin`

---

## Quick Reference

### Core Endpoints
| Endpoint | Method | Description |
|----------|--------|-------------|
| `/config` | CRUD | Database configuration |
| `/roles` | LIST | List roles |
| `/roles/:name` | CRUD | Role management |
| `/creds/:name` | GET | Generate credentials |
| `/rotate-root` | POST | Rotate root credentials |
| `/health` | GET | Health check |
| `/version` | GET | Plugin version |

### Key Files
| File | Purpose |
|------|---------|
| `plugin/backend.go` | Backend implementation |
| `plugin/path_config.go` | Configuration endpoint |
| `plugin/path_roles.go` | Role management |
| `plugin/path_creds.go` | Credential generation |
| `odbc/odbc.go` | ODBC connectivity |
| `storage/db_registry.go` | Database connection pool |

### Build and Test
```bash
# Build
make build

# Run tests
make test

# Run with coverage
make test-cover

# Run integration tests
make test-integration

# Lint
make lint
```

---

## Production Quality Requirements

> This should be in production. This should not have bugs or security issues. No shortcuts. If you find an issue or a bug, fix it, commit and push it, then continue with your work.

- All security vulnerabilities must be fixed before merging
- All bugs must be fixed before moving forward
- No TODO comments in production code
- Comprehensive error handling required
- Every error path must be tested

---

## Integration Tests - When and How

### Test Execution Order
1. **Unit Tests First** (80%+ coverage target)
   - Run with `go test ./...`
   - No external dependencies needed
   - Test all edge cases

2. **Integration Tests Last** (when unit tests exhausted)
   - Requires Teradata database running
   - Run with `make test-integration`

### Teradata Connection for Integration Tests
| Item | Value |
|------|-------|
| Host | testing-rhjbbw139fee5yg7.env.clearscape.teradata.com |
| User | demo_user |
| Password | latve1ja |
| DSN | Use ODBC driver |

### What Needs Integration Testing
- Full CREATE USER flow
- GRANT/REVOKE statements
- Credential renewal
- Credential revocation
- Connection pooling

---

## Documentation

- [API Reference](./docs/API.md)
- [Examples](./docs/EXAMPLES.md)
- [Troubleshooting](./docs/TROUBLESHOOTING.md)

---

## Tag Groups

| Tag | Description | Files |
|-----|-------------|-------|
| `#foundation` | Project setup, main.go, backend | `tasks/01-foundation.md` |
| `#core` | Core functionality, bug fixes | `tasks/02-core.md` |
| `#api` | API endpoints | `tasks/03-api.md` |
| `#security` | Security tests | `tasks/04-security.md` |
| `#tests` | Unit tests | `tasks/05-testing.md` |
| `#config` | Configuration | `tasks/02-core.md` |
| `#role` | Role management | `tasks/02-core.md` |
| `#credential` | Credential operations | `tasks/02-core.md` |
| `#odbc` | ODBC connectivity | `tasks/02-core.md` |

---

*For detailed task lists, see individual task files above.*
