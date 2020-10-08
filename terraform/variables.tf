# Copyright 2020 Google LLC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

variable "create_env_file" {
  type    = bool
  default = false

  description = "Create a .env file in the module directory with variables set to the configuration values."
}

variable "project" {
  type = string
}

variable "region" {
  type    = string
  default = "us-central1"
}

variable "database_tier" {
  type    = string
  default = "db-custom-8-30720"

  description = "Size of the Cloud SQL tier. Set to db-custom-1-3840 or a smaller instance for local dev."
}

variable "database_disk_size_gb" {
  type    = number
  default = 256

  description = "Size of the Cloud SQL disk, in GB."
}

variable "database_max_connections" {
  type    = number
  default = 100000

  description = "Maximum number of allowed connections. If you change to a smaller instance size, you must lower this number."
}

variable "database_name" {
  type    = string
  default = "en-verification"
}

variable "database_backup_location" {
  type    = string
  default = "us"

  description = "Location in which to backup the database."
}

variable "storage_location" {
  type    = string
  default = "US"
}

variable "redis_name" {
  type    = string
  default = "verification-cache"
}

# The location for the app engine; this implicitly defines the region for
# scheduler jobs as specified by the cloudscheduler_location variable but the
# values are sometimes different (as in the default values) so they are kept as
# separate variables.
# https://cloud.google.com/appengine/docs/locations
variable "appengine_location" {
  type    = string
  default = "us-central"
}

variable "cloudrun_location" {
  type    = string
  default = "us-central1"
}

# The cloudscheduler_location MUST use the same region as appengine_location but
# it must include the region number even if this is omitted from the
# appengine_location (as in the default values).
variable "cloudscheduler_location" {
  type    = string
  default = "us-central1"
}

variable "kms_location" {
  type    = string
  default = "us-central1"

  description = "Location in which to create KMS keys"
}

variable "kms_key_ring_name" {
  type    = string
  default = "verification"

  description = "Name of the KMS key ring to create"
}

variable "redis_location" {
  type    = string
  default = "us-central1-a"
}

variable "redis_alternative_location" {
  type    = string
  default = "us-central1-c"
}

variable "service_environment" {
  type    = map(map(string))
  default = {}

  description = "Per-service environment overrides."
}

variable "vpc_access_connector_max_throughput" {
  type    = number
  default = 1000

  description = "Maximum provisioned traffic throughput in Mbps"
}

variable "redis_cache_size" {
  type    = number
  default = 8

  description = "Size of the Redis instance in GB."
}

variable "server_hosts" {
  type    = list(string)
  default = []

  description = "List of domains upon which the web ui is served."
}

variable "apiserver_hosts" {
  type    = list(string)
  default = []

  description = "List of domains upon which the apiserver is served."
}

variable "adminapi_hosts" {
  type    = list(string)
  default = []

  description = "List of domains upon which the adminapi is served."
}

variable "enx_redirect_domain" {
  type        = string
  default     = ""
  description = "TLD for enx-redirect service links."
}

variable "enx_redirect_domain_map" {
  type = list(object({
    region = string
    host   = string
  }))
  default     = []
  description = "Redirect domains and environments."
}

variable "prevent_destroy" {
  type    = bool
  default = true

  description = "Prevent destruction of critical resources. Set this to false to actually destroy everything."
}

terraform {
  required_version = ">= 0.13.1"

  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 3.38"
    }
    google-beta = {
      source  = "hashicorp/google-beta"
      version = "~> 3.38"
    }
    local = {
      source  = "hashicorp/local"
      version = "~> 1.4"
    }
    null = {
      source  = "hashicorp/null"
      version = "~> 2.1"
    }
    random = {
      source  = "hashicorp/random"
      version = "~> 2.3"
    }
  }
}
