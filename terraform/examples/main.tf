terraform {
  required_version = ">= 1.0"

  required_providers {
    vault = {
      source  = "hashicorp/vault"
      version = "~> 3.0"
    }
  }

  backend "local" {
    path = "terraform.tfstate"
  }
}

provider "vault" {
  address = "http://localhost:8200"
  token   = "root-token"
}

resource "vault_teradata_database_plugin_connection_config" "teradata" {
  mount = "teradata"

  connection_string = "DRIVER={Teradata};DBCNAME=teradata.example.com;UID=dbadmin;PWD=secretpassword"

  max_open_connections    = 10
  max_idle_connections    = 5
  connection_timeout      = 30
  verify_connection       = true
}

resource "vault_teradata_database_plugin_role" "readonly" {
  mount = "teradata"
  name  = "readonly"

  db_user           = "vault_readonly_${uuid()}"
  default_ttl       = 3600
  max_ttl           = 86400
  default_database  = "mydb"

  creation_statement = <<-EOT
    GRANT SELECT ON mydb TO {{username}};
  EOT

  revocation_statement = <<-EOT
    REVOKE ALL ON mydb FROM {{username}};
  EOT

  rollback_statement = <<-EOT
    DROP USER {{username}};
  EOT
}

resource "vault_teradata_database_plugin_role" "readwrite" {
  mount = "teradata"
  name  = "readwrite"

  db_user           = "vault_readwrite_${uuid()}"
  default_ttl       = 3600
  max_ttl           = 86400
  default_database  = "mydb"

  creation_statement = <<-EOT
    GRANT SELECT, INSERT, UPDATE, DELETE ON mydb TO {{username}};
  EOT

  revocation_statement = <<-EOT
    REVOKE ALL ON mydb FROM {{username}};
  EOT

  rollback_statement = <<-EOT
    DROP USER {{username}};
  EOT
}

resource "vault_teradata_database_plugin_role" "app" {
  mount = "teradata"
  name  = "app"

  db_user           = "vault_app_${uuid()}"
  default_ttl       = 7200
  max_ttl           = 14400
  default_database  = "appdb"

  perm_space        = 1000000000
  spool_space       = 2000000000
  fallback          = true

  creation_statement = <<-EOT
    GRANT ALL ON appdb TO {{username}};
  EOT

  revocation_statement = <<-EOT
    REVOKE ALL ON appdb FROM {{username}};
  EOT

  rollback_statement = <<-EOT
    DROP USER {{username}};
  EOT
}

data "vault_teradata_database_plugin_credentials" "app_creds" {
  mount = "teradata"
  name  = "app"
}

output "app_username" {
  description = "Application database username"
  value       = data.vault_teradata_database_plugin_credentials.app_creds.username
}

output "app_password" {
  description = "Application database password"
  value       = data.vault_teradata_database_plugin_credentials.app_creds.password
  sensitive   = true
}
