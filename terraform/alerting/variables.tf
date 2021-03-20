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
    window = string
  }))

  default = {
    // appsync runs every 4h, alert after 2 failures
    "appsync" = { metric = "appsync/success", window = "485m" },

    // cleanup runs every 1h, alert after 4 failures
    "cleanup" = { metric = "cleanup/success", window = "245m" },

    // modeler runs every 4h, alert after 2 failures
    "modeler" = { metric = "modeler/success", window = "485m" },

    // realm-key-rotation runs every 15m, alert after 2 failures
    "realm-key-rotation" = { metric = "rotation/verification/success", window = "35m" }

    // rotation runs every 30m, alert after 2 failures
    "rotation" = { metric = "rotation/token/success", window = "65m" }

    // stats-puller runs every 15m, alert after 2 failures
    "stats-puller" = { metric = "statspuller/success", window = "35m" }
  }
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
