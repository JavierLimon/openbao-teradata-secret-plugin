resource "vault_teradata_database_plugin_connection_config" "this" {
  mount = var.teradata_mount_point

  connection_string = "DRIVER={Teradata};DBCNAME=${var.teradata_host};UID=${var.teradata_root_user};PWD=${var.teradata_root_password}"

  max_open_connections    = var.max_open_connections
  max_idle_connections    = var.max_idle_connections
  connection_timeout      = var.connection_timeout
  verify_connection       = var.verify_connection
}

variable "teradata_host" {
  description = "Teradata database hostname"
  type        = string
}

variable "teradata_root_user" {
  description = "Teradata root username for connection"
  type        = string
}

variable "teradata_root_password" {
  description = "Teradata root password for connection"
  type        = string
  sensitive   = true
}

variable "max_open_connections" {
  description = "Maximum number of open connections"
  type        = number
  default     = 10
}

variable "max_idle_connections" {
  description = "Maximum number of idle connections"
  type        = number
  default     = 5
}

variable "connection_timeout" {
  description = "Connection timeout in seconds"
  type        = number
  default     = 30
}

variable "verify_connection" {
  description = "Verify connection on configuration"
  type        = bool
  default     = true
}
