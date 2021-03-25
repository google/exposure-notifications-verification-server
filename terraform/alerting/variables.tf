variable "project" {
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

variable "alert-notification-channel-paging" {
  type = map(any)
  default = {
    email = {
      labels = {
        email_address = "nobody@example.com"
      }
    }
    slack = {
      labels = {
        channel_name = "#paging-channel"
        auth_token   = "paging-token"
      }
    }
  }
  description = "Paging notification channel"
}

variable "alert-notification-channel-non-paging" {
  type = map(any)
  default = {
    email = {
      labels = {
        email_address = "nobody@example.com"
      }
    }
    slack = {
      labels = {
        channel_name = "#non-paging-channel"
        auth_token   = "non-paging channel"
      }
    }
  }
  description = "Non-paging notification channel"
}

variable "slo_thresholds_overrides" {
  type    = map(any)
  default = {}
}

variable "alert_on_human_accessed_secret" {
  type    = bool
  default = true

  description = "Alert when a human accesses a secret. You must enable DATA_READ audit logs for Secret Manager."
}

variable "alert_on_human_decrypted_value" {
  type    = bool
  default = true

  description = "Alert when a human accesses a secret. You must enable DATA_READ audit logs for Cloud KMS."
}

variable "forward_progress_indicators" {
  type = map(object({
    metric = string
    window = number
  }))

  description = "Map of overrides for forward progress indicators. These are merged with the default variables. The window must be in seconds."
  default     = {}
}

terraform {
  required_version = ">= 0.14.2"

  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 3.51"
    }
    google-beta = {
      source  = "hashicorp/google-beta"
      version = "~> 3.51"
    }
  }
}
