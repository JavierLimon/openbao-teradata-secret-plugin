# Testing Tasks - Teradata Secret Plugin

Unit and integration testing.

## Status
- Total: 8
- Completed: 0
- Pending: 8

---

## Test Coverage Targets

| Package | Target | Current |
|---------|--------|---------|
| plugin | 80% | 0% |
| storage | 60% | 0% |
| odbc | 70% | 0% |
| **Overall** | **80%** | **0%** |

---

## T-026: Unit Tests - Backend

**Priority**: HIGH | **Status**: pending

Test backend initialization.

### Sub-tasks
- [ ] T-026.1: Test Factory function
- [ ] T-026.2: Test path registration
- [ ] T-026.3: Test backend setup

---

## T-027: Unit Tests - Config

**Priority**: HIGH | **Status**: pending

Test configuration paths.

### Sub-tasks
- [ ] T-027.1: Test config write
- [ ] T-027.2: Test config read
- [ ] T-027.3: Test config delete
- [ ] T-027.4: Test config validation

---

## T-028: Unit Tests - Roles

**Priority**: HIGH | **Status**: pending

Test role management.

### Sub-tasks
- [ ] T-028.1: Test role CRUD
- [ ] T-028.2: Test role listing
- [ ] T-028.3: Test role validation

---

## T-029: Unit Tests - Credentials

**Priority**: HIGH | **Status**: pending

Test credential generation.

### Sub-tasks
- [ ] T-029.1: Test username generation
- [ ] T-029.2: Test password generation
- [ ] T-029.3: Test SQL building
- [ ] T-029.4: Test error handling

---

## T-030: Unit Tests - ODBC

**Priority**: MEDIUM | **Status**: pending

Test ODBC module.

### Sub-tasks
- [ ] T-030.1: Test connection
- [ ] T-030.2: Test query execution
- [ ] T-030.3: Mock for CI/CD

---

## T-031: Integration Tests

**Priority**: HIGH | **Status**: pending

Test with real Teradata database.

### Sub-tasks
- [ ] T-031.1: Test CREATE USER flow
- [ ] T-031.2: Test GRANT flow
- [ ] T-031.3: Test REVOKE flow
- [ ] T-031.4: Test DROP USER flow
- [ ] T-031.5: Test credential renewal

### Test Environment
```
Host: testing-rhjbbw139fee5yg7.env.clearscape.teradata.com
User: demo_user
Password: latve1ja
```

---

## T-032: Mock Tests

**Priority**: MEDIUM | **Status**: pending

Create mocks for testing without database.

### Sub-tasks
- [ ] T-032.1: Mock ODBC connection
- [ ] T-032.2: Mock SQL results
- [ ] T-032.3: Use mocks in unit tests

---

## T-033: Coverage Report

**Priority**: MEDIUM | **Status**: pending

Generate and track coverage.

### Sub-tasks
- [ ] T-033.1: Set up coverage tooling
- [ ] T-033.2: Generate reports
- [ ] T-033.3: Track in docs

---

## Running Tests

```bash
# Unit tests
go test -v ./...

# With coverage
go test -cover ./...

# Specific package
go test -v ./plugin/...

# Integration tests
export RUN_INTEGRATION_TESTS=true
go test -tags=integration -v ./plugin/...
```

---

## Test Categories

| Category | Files |
|----------|-------|
| Backend | backend_test.go |
| Config | config_test.go |
| Roles | roles_test.go |
| Credentials | creds_test.go |
| ODBC | odbc_test.go |
| Integration | integration_test.go |
