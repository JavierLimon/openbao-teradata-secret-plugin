data "vault_teradata_database_plugin_credentials" "readonly" {
  mount = var.teradata_mount_point
  name  = "readonly"
}

data "vault_teradata_database_plugin_credentials" "readwrite" {
  mount = var.teradata_mount_point
  name  = "readwrite"
}

data "vault_teradata_database_plugin_credentials" "app" {
  mount = var.teradata_mount_point
  name  = "app"
}

output "readonly_credentials" {
  description = "Read-only role credentials"
  value       = {
    username = data.vault_teradata_database_plugin_credentials.readonly.username
    password = data.vault_teradata_database_plugin_credentials.readonly.password
  }
  sensitive = true
}

output "readwrite_credentials" {
  description = "Read-write role credentials"
  value       = {
    username = data.vault_teradata_database_plugin_credentials.readwrite.username
    password = data.vault_teradata_database_plugin_credentials.readwrite.password
  }
  sensitive = true
}

output "app_credentials" {
  description = "App role credentials"
  value       = {
    username = data.vault_teradata_database_plugin_credentials.app.username
    password = data.vault_teradata_database_plugin_credentials.app.password
  }
  sensitive = true
}
