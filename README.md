# OpenBAO Teradata Secret Plugin

Secret plugin for OpenBAO that provides dynamic database credentials for Teradata databases using ODBC.

## Status

**In Development** - Foundation phase complete

## Quick Start

```bash
# Build
make build

# Test
make test

# Run
./bin/openbao-teradata-secret-plugin
```

## Features

- Dynamic database credential generation
- ODBC connectivity for Teradata
- Role-based access control
- SQL statement templates
- Connection pooling
- Credential rotation support

## Configuration

```bash
# Configure Teradata connection
bao write teradata/config \
    connection_string="DSN=MyTeradata;UID=admin;PWD=password" \
    max_open_connections=5 \
    max_idle_connections=2
```

## Role Management

```bash
# Create a role
bao write teradata/roles/readonly \
    db_user="vault_user_{{uuid}}" \
    creation_statement="CREATE USER {{username}} FROM {{password}};" \
    default_ttl=3600 \
    max_ttl=86400

# Generate credentials
bao read teradata/creds/readonly
```

## Documentation

- [INSTRUCTIONS.md](./INSTRUCTIONS.md) - AI Agent instructions
- [API.md](./docs/API.md) - API reference

---

*Generated: 2026-03-20*
