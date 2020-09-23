variable "project" {
  type = string
}

variable "notification-email" {
  type        = string
  default     = "nobody@example.com"
  description = "Email address for alerts to go to."
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
