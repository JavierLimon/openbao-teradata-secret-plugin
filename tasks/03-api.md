# API Tasks - Teradata Secret Plugin

API endpoint implementation.

## Status
- Total: 5
- Completed: 0
- Pending: 5

---

## T-015: Root Credential Rotation

**Priority**: HIGH | **Status**: pending

Implement root credential rotation.

### Sub-tasks
- [ ] T-015.1: Add `/rotate-root` endpoint
- [ ] T-015.2: Generate new root password
- [ ] T-015.3: Update configuration

### Dependencies
- T-007 (Connection Pool)

---

## T-016: Static Credentials

**Priority**: MEDIUM | **Status**: pending

Support static username/password.

### Sub-tasks
- [ ] T-016.1: Add static credential type
- [ ] T-016.2: Implement password rotation for static
- [ ] T-016.3: Add to role model

---

## T-017: Batch Credentials

**Priority**: MEDIUM | **Status**: pending

Generate multiple credentials at once.

### Sub-tasks
- [ ] T-017.1: Add `/creds/batch/:name` endpoint
- [ ] T-017.2: Handle multiple user creation
- [ ] T-017.3: Return batch results

---

## T-018: Connection Status

**Priority**: LOW | **Status**: pending

Enhanced connection status.

### Sub-tasks
- [ ] T-018.1: Add connection pool stats
- [ ] T-018.2: Show active connections
- [ ] T-018.3: Add connection errors

---

## T-019: Statement Listing

**Priority**: LOW | **Status**: pending

List stored statements.

### Sub-tasks
- [ ] T-019.1: Add `/statements` list endpoint
- [ ] T-019.2: Filter by type
- [ ] T-019.3: Pagination support

---

## API Endpoints Summary

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/config` | GET/POST/DELETE | Database configuration |
| `/roles` | LIST | List roles |
| `/roles/:name` | GET/POST/PUT/DELETE | Role CRUD |
| `/creds/:name` | GET | Generate credentials |
| `/creds/batch/:name` | POST | Batch credentials |
| `/rotate-root` | POST | Rotate root |
| `/statements` | LIST | List statements |
| `/statements/:name` | CRUD | Statement CRUD |
| `/health` | GET | Health check |
| `/version` | GET | Plugin version |
