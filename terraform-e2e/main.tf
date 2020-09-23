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

resource "random_string" "suffix" {
  length  = 5
  special = false
  number  = false
  upper   = false
}

module "en" {
  source = "../terraform"

  project           = var.project
  database_name     = "en-verification-${random_string.suffix.result}"
  redis_name        = "verification-cache-${random_string.suffix.result}"
  kms_key_ring_name = "verification-${random_string.suffix.result}"

  create_env_file = true

  service_environment = {
    server = {
      FIREBASE_PRIVACY_POLICY_URL   = "https://policies.google.com/privacy"
      FIREBASE_TERMS_OF_SERVICE_URL = "https://policies.google.com/terms"
      LOG_DEBUG = "true"
    }

    apiserver = {
      LOG_DEBUG = "true"
    }

    adminapi = {
      LOG_DEBUG = "true"
    }

    e2e-runner = {
      HEALTH_AUTHORITY_CODE = "e2e-test-only"
      KEY_SERVER = "TODO"
      LOG_DEBUG = "true"
    }
  }
}

output "en" {
  value = module.en
}
