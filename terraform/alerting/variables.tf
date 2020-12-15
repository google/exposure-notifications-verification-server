variable "monitoring-host-project" {
  type        = string
  description = <<-EOT
  The host project for multi-project workspace. See also:
  http://cloud/monitoring/workspaces/create#first-multi-project-workspace If
  empty, will use var.verificatin-server-project by default
  EOT
}

variable "verification-server-project" {
  type        = string
  description = "GCP project for verification server. Required."
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

variable "https-forwarding-rule" {
  type        = string
  default     = "verification-server-https"
  description = "GCP Cloud Load Balancer forwarding rule name."
}

variable "alert-notification-channels" {
  type = map(any)
  default = {
    email = {
      labels = {
        email_address = "nobody@example.com"
      }
    }
    slack = {
      labels = {
        channel_name = "#foo"
        auth_token   = "abr"
      }
    }
  }
  description = "Notification channels"
}

variable "slo_thresholds_overrides" {
  type    = map(any)
  default = {}
}



terraform {
  required_version = ">= 0.14.2"

  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 3.46"
    }
    google-beta = {
      source  = "hashicorp/google-beta"
      version = "~> 3.46"
    }
  }
}
