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
  redis_name        = "verification-cache-${random_string.suffix.result}"
  kms_key_ring_name = "verification-${random_string.suffix.result}"

  create_env_file = true

  service_environment = {
    server = {
      FIREBASE_PRIVACY_POLICY_URL   = "https://policies.google.com/privacy"
      FIREBASE_TERMS_OF_SERVICE_URL = "https://policies.google.com/terms"

      LOG_LEVEL = "debug"
    }

    apiserver = {
      LOG_LEVEL = "debug"
    }

    adminapi = {
      LOG_LEVEL = "debug"
    }

    e2e-runner = {
      HEALTH_AUTHORITY_CODE = "e2e-test-only"
      KEY_SERVER            = "TODO"

      LOG_LEVEL = "debug"
    }
  }
}

module "en-alerting" {
  source = "../terraform/alerting"

  project = var.project

  adminapi_hosts  = ["adminapi.notexistdomain.notexist"]
  apiserver_hosts = ["apiserver.notexistdomain.notexist"]
  server_hosts    = ["encv-test.notexistdomain.notexist"]

  depends_on = [module.en]
}

output "en" {
  value = module.en
}
