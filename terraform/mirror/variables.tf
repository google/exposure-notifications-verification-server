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

variable "artifact_registry_location" {
  type    = string
  default = "us"

  description = "Location for Artifact Registry repositories."
}

variable "repository_readers" {
  type    = list(string)
  default = []

  description = "List of entities that should be able to pull from the mirror."
}

variable "cloudscheduler_location" {
  type    = string
  default = "us-central1"
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
