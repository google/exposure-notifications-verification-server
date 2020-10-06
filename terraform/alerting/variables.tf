variable "monitoring-host-project" {
  type        = string
  default     = ""
  description = "The host project for multi-project workspace. See also: http://cloud/monitoring/workspaces/create#first-multi-project-workspace If empty, will use var.verificatin-server-project by default"
}

variable "verification-server-project" {
  type        = string
  description = "GCP project for verification server. Required."
}

variable "notification-email" {
  type        = string
  default     = "nobody@example.com"
  description = "Email address for alerts to go to."
}

variable "server_hosts" {
  type        = list(string)
  description = "List of domains upon which the web ui is served."
}

variable "apiserver_hosts" {
  type        = list(string)
  description = "List of domains upon which the apiserver is served."
}

variable "adminapi_hosts" {
  type        = list(string)
  description = "List of domains upon which the adminapi is served."
}

variable "extra-hosts" {
  type        = list(string)
  default     = []
  description = "Extra hosts to probe and monitor."
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
