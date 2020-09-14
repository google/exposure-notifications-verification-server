variable "monitoring-host-project" {
  type        = string
  description = "The host project for multi-project workspace. See also: http://cloud/monitoring/workspaces/create#first-multi-project-workspace"
}

variable "notification-email" {
  type        = string
  default     = "nobody@example.com"
  description = "Email address for alerts"
}

variable "server-host" {
  type        = string
  default     = ""
  description = "Domain web ui is hosted on."
}

variable "apiserver-host" {
  type        = string
  default     = ""
  description = "Domain apiserver is hosted on."
}

variable "adminapi-host" {
  type        = string
  default     = ""
  description = "Domain adminapi is hosted on."
}

terraform {
  required_version = ">= 0.13"

  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 3.36"
    }
    google-beta = {
      source  = "hashicorp/google-beta"
      version = "~> 3.36"
    }
  }
}
