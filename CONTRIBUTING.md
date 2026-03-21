# Contributing to OpenBAO Teradata Secret Plugin

Thank you for your interest in contributing to the OpenBAO Teradata Secret Plugin.

## Development Setup

### Prerequisites

- Go 1.24 or later
- ODBC driver for Teradata
- golangci-lint (for linting)

### Clone and Setup

```bash
git clone https://github.com/JavierLimon/openbao-teradata-secret-plugin.git
cd openbao-teradata-secret-plugin
go mod download
```

### Build

```bash
make build
```

The binary is output to `bin/openbao-teradata-secret-plugin`.

## Testing

### Run All Tests

```bash
make test
```

### Run Unit Tests Only

```bash
make test-unit
```

### Run with Coverage

```bash
make test-cover
```

View coverage report:
```bash
make coverage
```

### Run Integration Tests

Integration tests require a Teradata database:

```bash
make test-integration
```

Set environment variables for Teradata connection:
```bash
export TERADATA_HOST=your-host
export TERADATA_USER=your-user
export TERADATA_PASSWORD=your-password
export RUN_INTEGRATION_TESTS=true
make test-integration
```

## Linting

```bash
make lint
```

## Code Structure

```
.
├── cmd/teradata/      # Main entry point
├── plugin/            # Vault plugin paths (config, roles, creds)
├── odbc/              # ODBC connectivity
├── models/            # Data models
├── storage/           # Storage and connection pooling
├── security/          # Security utilities
└── audit/             # Audit logging
```

## Making Changes

1. Create a feature branch
2. Make your changes
3. Run tests and linting
4. Submit a pull request

## Plugin Registration

To use with OpenBao, register the plugin:

```bash
bao plugin register -version=0.1.0 github.com/JavierLimon/openbao-teradata-secret-plugin
bao secrets enable -path=teradata teradata
```
