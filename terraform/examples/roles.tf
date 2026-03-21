resource "vault_teradata_database_plugin_role" "readonly" {
  mount = var.teradata_mount_point
  name  = "readonly"

  db_user           = "vault_readonly_${var.username_suffix}"
  default_ttl       = 3600
  max_ttl           = 86400
  default_database  = var.database_name
  creation_statement = <<-EOT
    GRANT SELECT ON ${var.database_name} TO {{username}};
  EOT

  revocation_statement = <<-EOT
    REVOKE ALL ON ${var.database_name} FROM {{username}};
  EOT

  rollback_statement = <<-EOT
    DROP USER {{username}};
  EOT
}

resource "vault_teradata_database_plugin_role" "readwrite" {
  mount = var.teradata_mount_point
  name  = "readwrite"

  db_user           = "vault_readwrite_${var.username_suffix}"
  default_ttl       = 3600
  max_ttl           = 86400
  default_database  = var.database_name
  creation_statement = <<-EOT
    GRANT SELECT, INSERT, UPDATE, DELETE ON ${var.database_name} TO {{username}};
  EOT

  revocation_statement = <<-EOT
    REVOKE ALL ON ${var.database_name} FROM {{username}};
  EOT

  rollback_statement = <<-EOT
    DROP USER {{username}};
  EOT
}

resource "vault_teradata_database_plugin_role" "app" {
  mount = var.teradata_mount_point
  name  = "app"

  db_user           = "vault_app_${var.username_suffix}"
  default_ttl       = 7200
  max_ttl           = 14400
  default_database  = var.database_name

  perm_space        = 1000000000
  spool_space       = 2000000000
  fallback          = true

  creation_statement = <<-EOT
    GRANT ALL ON ${var.database_name} TO {{username}};
  EOT

  revocation_statement = <<-EOT
    REVOKE ALL ON ${var.database_name} FROM {{username}};
  EOT

  rollback_statement = <<-EOT
    DROP USER {{username}};
  EOT
}

variable "username_suffix" {
  description = "Suffix for generated usernames"
  type        = string
  default     = ""
}

variable "database_name" {
  description = "Default database for roles"
  type        = string
}
