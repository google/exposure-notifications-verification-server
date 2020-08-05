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

variable "adminapi_custom_domain" {
  type    = string
  default = ""

  description = "Custom domain to map for adminapi. This domain must already be verified by Google, and you must have a DNS CNAME record pointing to ghs.googlehosted.com in advance. If not provided, no domain mapping is created."
}

variable "apiserver_custom_domain" {
  type    = string
  default = ""

  description = "Custom domain to map for apiserver. This domain must already be verified by Google, and you must have a DNS CNAME record pointing to ghs.googlehosted.com in advance. If not provided, no domain mapping is created."
}

variable "server_custom_domain" {
  type    = string
  default = ""

  description = "Custom domain to map for server. This domain must already be verified by Google, and you must have a DNS CNAME record pointing to ghs.googlehosted.com in advance. If not provided, no domain mapping is created."
}

terraform {
  required_providers {
    google      = "~> 3.32"
    google-beta = "~> 3.32"
    null        = "~> 2.1"
    random      = "~> 2.3"
  }
}
