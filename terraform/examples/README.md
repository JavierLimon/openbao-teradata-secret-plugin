# Terraform Provider Examples for Teradata Secrets

This directory contains Terraform configurations for managing Teradata secrets using the OpenBao Teradata Secret Plugin.

## Prerequisites

- Terraform >= 1.0
- OpenBao server running
- Teradata secrets engine enabled at `teradata` path
- Vault provider token with appropriate permissions

## Quick Start

1. Copy the example variables file:

```bash
cp terraform.tfvars.example terraform.tfvars
```

2. Edit `terraform.tfvars` with your actual values:

```hcl
vault_address    = "http://localhost:8200"
vault_token      = "your-vault-token"
teradata_host    = "teradata.example.com"
teradata_root_user = "dbadmin"
teradata_root_password = "your-password"
database_name    = "mydb"
```

3. Initialize Terraform:

```bash
terraform init
```

4. Plan and apply:

```bash
terraform plan
terraform apply
```

## Files Overview

| File | Description |
|------|-------------|
| `main.tf` | Complete example with all resources |
| `provider.tf` | Provider configuration with variables |
| `config.tf` | Database connection configuration |
| `roles.tf` | Role definitions (readonly, readwrite, app) |
| `credentials.tf` | Data sources for credential retrieval |
| `terraform.tfvars.example` | Example variable values |

## Resources

### vault_teradata_database_plugin_connection_config

Configures the connection to the Teradata database.

### vault_teradata_database_plugin_role

Creates dynamic database roles with:
- TTL settings (default/max)
- Database permissions
- Space limits (PERM/SPOOL)
- Creation, revocation, and rollback statements

### vault_teradata_database_plugin_credentials

Retrieves dynamic credentials for a configured role.

## Outputs

| Output | Description |
|--------|-------------|
| `app_username` | Generated database username |
| `app_password` | Generated database password (sensitive) |

## Usage with Application

### As Environment Variables

```bash
export DB_HOST="teradata.example.com"
export DB_USER="$(terraform output -raw app_username)"
export DB_PASS="$(terraform output -raw app_password)"
```

### In Kubernetes Secret

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: teradata-creds
type: Opaque
stringData:
  username: <app_username output>
  password: <app_password output>
```

## Rotating Credentials

Credentials are automatically rotated based on the role's TTL. To manually rotate:

```bash
terraform apply -var="teradata_root_password=newpassword"
```

Or rotate root credentials via OpenBao:

```bash
bao write teradata/rotate-root
```
