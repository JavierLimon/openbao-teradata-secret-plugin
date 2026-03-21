terraform {
  required_providers {
    bao = {
      source  = "hashicorp/vault"
      version = "~> 3.0"
    }
  }
}

provider "vault" {
  address = var.vault_address
  token   = var.vault_token

  skip_tls_verify = var.skip_tls_verify
}

variable "vault_address" {
  description = "URL of the Vault server"
  type        = string
  default     = "http://localhost:8200"
}

variable "vault_token" {
  description = "Vault token with permissions for the teradata secrets engine"
  type        = string
  sensitive   = true
}

variable "skip_tls_verify" {
  description = "Skip TLS verification for Vault connection"
  type        = bool
  default     = false
}

variable "teradata_mount_point" {
  description = "Mount point for the Teradata secrets engine"
  type        = string
  default     = "teradata"
}
