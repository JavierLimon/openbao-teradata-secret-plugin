# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.0] - 2026-03-21

### Added
- Initial commit: OpenBAO Teradata Secret Plugin
- ODBC connection pool with health checks
- Secure credential generation with crypto/rand
- CREATE USER SQL in odbc package
- GRANT statement execution in ODBC package
- Statement templates CRUD with proper storage
- SQL injection prevention with username validation
- Batch credentials endpoint for generating multiple credentials
- Connection string security validation and masking
- Password security requirements: minimum 16 chars with uppercase, lowercase, numbers, and special characters
- Connection pool stats endpoint
- Audit logging for credential operations
- Credential renewal and revocation endpoints
- Unit tests for credential generation
- Role TTL and renewal settings
- Unit tests for role CRUD operations
- Integration tests for credential lifecycle
- Config and role validation tests
- Docker-compose for local development with Teradata and OpenBAO
- Dockerfile for plugin containerization
- GitHub Actions CI workflow with build, test, lint jobs
- Terraform provider examples for Teradata secrets
- CONTRIBUTING.md with development setup and testing instructions
