# Teradata Secret Plugin - AI Agent Instructions

## Project Overview
- **Project Name**: openbao-teradata-secret-plugin
- **Type**: OpenBAO Secret Plugin
- **Purpose**: Dynamic database credentials for Teradata using ODBC

## Status
- **Phase**: Foundation (Initial Setup)
- **Maturity Target**: Production-ready

## Reference Projects
- **transform**: 100% maturity, reference implementation for patterns
- **kmip**: 70% maturity, security patterns
- **plugin-cf**: 50% maturity, auth patterns

## Quick Start

```bash
# Install dependencies
go mod download

# Build
make build

# Test
make test

# Run
./bin/openbao-teradata-secret-plugin
```

## Phase 1: Foundation

### Tasks
- [x] T1.1: Create main.go with plugin.ServeMultiplex
- [x] T1.2: Implement Backend factory
- [x] T1.3: Add Configuration path (/config)
- [ ] T1.4: Add Role management paths
- [ ] T1.5: Add Credentials generation path

### Verification
```bash
go test ./plugin/ -v
```

## Phase 2: Core Features

### Tasks
- [ ] T2.1: Implement role CRUD operations
- [ ] T2.2: Implement dynamic credential generation
- [ ] T2.3: Implement SQL statement templates
- [ ] T2.4: Add connection pool management

## Phase 3: Testing

### Tasks
- [ ] T3.1: Write unit tests (80%+ coverage target)
- [ ] T3.2: Write integration tests
- [ ] T3.3: Test with real Teradata database

## API Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/config` | GET/POST/DELETE | Database configuration |
| `/roles` | LIST | List all roles |
| `/roles/:name` | GET/POST/PUT/DELETE | Role CRUD |
| `/creds/:name` | GET | Generate credentials |
| `/rotate-root` | POST | Rotate root credentials |
| `/health` | GET | Health check |
| `/version` | GET | Plugin version |

## Key Files

```
plugin/
├── backend.go          # Backend implementation
├── path_config.go      # Configuration endpoint
├── path_roles.go       # Role management
├── path_creds.go      # Credential generation
├── path_operational.go # Health/version
└── version.go        # Version info

storage/
└── db_registry.go    # Database connection pool

models/
└── config.go        # Data models
```

## Next Steps

1. Run `go mod tidy` to fetch dependencies
2. Run `make build` to verify compilation
3. Run `make test` to run unit tests
4. Review and implement remaining path handlers

## Notes

- Uses ODBC for Teradata connectivity
- Follows database secrets engine patterns
- Supports dynamic credential generation
- Connection pooling via sql.DB
