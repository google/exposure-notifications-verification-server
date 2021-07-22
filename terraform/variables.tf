# Copyright 2020 the Exposure Notifications Verification Server authors
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

variable "redirect_cert_generation" {
  type    = string
  default = ""

  description = "generation of the ENX redirect certificate. Needs to be incremented when new subdomains are added."
}

variable "redirect_cert_generation_next" {
  type    = string
  default = ""

  description = "generation of the ENX redirect certificate. Needs to be incremented when new subdomains are added."
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

variable "database_version" {
  type    = string
  default = "POSTGRES_13"

  description = "Version of the database to use. Must be at least 13 or higher."
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

variable "database_backup_location" {
  type    = string
  default = "us"

  description = "Location in which to backup the database."
}

variable "database_backup_schedule" {
  type    = string
  default = "0 */6 * * *"

  description = "Cron schedule in which to do a full backup of the database to Cloud Storage."
}

variable "database_failover_replica_regions" {
  type    = list(string)
  default = []

  description = "List of regions in which to create failover replicas. The default configuration is resistant to zonal outages. This will increase costs."
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

variable "redis_enable_auth" {
  type    = bool
  default = false

  description = "Enable Redis authentication. The default is false because not all redis versions support authentication."
}

variable "service_environment" {
  type    = map(map(string))
  default = {}

  description = "Per-service environment overrides The special key \"_all\" will apply to all services. This is useful for common configuration like log-levels. A service-specific configuration overrides a value in \"_all\"."
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

variable "enx_redirect_domain_map_add" {
  type = list(object({
    region = string
    host   = string
  }))
  default     = []
  description = "Redirect domains and environments to be added to the next cert."
}

variable "db_phone_number_hmac_count" {
  type    = number
  default = 1

  description = "Number of HMAC keys to create for HMACing phone numbers in the database. Increase by 1 to force a rotation."
}

variable "db_apikey_db_hmac_count" {
  type    = number
  default = 1

  description = "Number of HMAC keys to create for HMACing API keys in the database. Increase by 1 to force a rotation."
}

variable "db_apikey_sig_hmac_count" {
  type    = number
  default = 1

  description = "Number of HMAC keys to create for HMACing API key signatures. Increase by 1 to force a rotation."
}

variable "db_verification_code_hmac_count" {
  type    = number
  default = 1

  description = "Number of HMAC keys to create for HMACing verification codes in the database. Increase by 1 to force a rotation."
}

variable "enable_lb_logging" {
  type        = bool
  default     = false
  description = <<-EOT
  Whether to enable load balancer logging. This is useful for debugging Cloud
  Armor issues.
  EOT
}

variable "log_retention_period" {
  type        = number
  default     = 14
  description = "Number of days to retain logs for all services in the project"
}

// Note: in Cloud Run/Knative, there are two kinds of annotations.
// - Service level annotations: applies to all revisions in the service. E.g.
//   the ingress restriction
//   https://cloud.google.com/run/docs/securing/ingress#yaml
// - Revision level annotations: only applies to a new revision you want to
//   create. E.g. the VPC connector setting
//   https://cloud.google.com/run/docs/configuring/connecting-vpc#yaml
//
// Unfortunately they are just too similar and you'll have to read the doc
// carefully to know what kind of annotation is needed to enable a feature.
//
// The variables below are named service_annotations and revision_annotations
// accordingly.

locals {
  default_revision_annotations = {
    "autoscaling.knative.dev/maxScale" : "2",
    "run.googleapis.com/sandbox" : "gvisor"
    "run.googleapis.com/vpc-access-connector" : google_vpc_access_connector.connector.id
    "run.googleapis.com/vpc-access-egress" : "private-ranges-only"
  }
  default_service_annotations = {
    "run.googleapis.com/binary-authorization" : "default"
    "run.googleapis.com/ingress" : "all"
    // This is added due to the run.googleapis.com/sandbox annotation above.
    // The sandbox anntation it added to remove the permanent diff.
    "run.googleapis.com/launch-stage" : "BETA"
  }
}

variable "service_annotations" {
  type    = map(map(string))
  default = {}

  description = "Per-service service level annotations."
}

variable "default_service_annotations_overrides" {
  type    = map(string)
  default = {}

  description = <<-EOT
  Annotations that applies to all services. Can be used to override
  default_service_annotations.
  EOT
}

variable "revision_annotations" {
  type    = map(map(string))
  default = {}

  description = "Per-service revision level annotations."
}

variable "default_revision_annotations_overrides" {
  type    = map(string)
  default = {}

  description = <<-EOT
  Annotations that applies to all services. Can be used to override
  default_revision_annotations.
  EOT
}

variable "binary_authorization_enforcement_mode" {
  type    = string
  default = "ENFORCED_BLOCK_AND_AUDIT_LOG"

  description = "Binary authorization enforcement mechanism, must be one of ENFORCED_BLOCK_AND_AUDIT_LOG or DRYRUN_AUDIT_LOG_ONLY"
}

variable "binary_authorization_allowlist_patterns" {
  type    = set(string)
  default = []

  description = "List of container references to always allow, even without attestations."
}

variable "e2e_skip_sms" {
  type    = bool
  default = false

  description = "Skip SMS tests when executing the e2e runner. Set this to true to not send SMS. You must also configure the e2e realm with proper test credentials."
}

terraform {
  required_version = ">= 0.14.2"

  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 3.55"
    }
    google-beta = {
      source  = "hashicorp/google-beta"
      version = "~> 3.55"
    }
    local = {
      source  = "hashicorp/local"
      version = "~> 2.0"
    }
    null = {
      source  = "hashicorp/null"
      version = "~> 3.0"
    }
    random = {
      source  = "hashicorp/random"
      version = "~> 2.3"
    }
  }
}
