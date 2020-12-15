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

locals {
  default_slo_thresholds = {
    adminapi = {
      availability = 0.995
      latency      = 0.9
    }
    apiserver = {
      availability = 0.995
      latency      = 0.9
    }
    appsync = {
      availability = 0.995
      latency      = 0.9
    }
    cleanup = {
      availability = 0.995
      latency      = 0.9
    }
    e2e-runner = {
      availability = 0.995
      latency      = 0.9
    }
    enx-redirect = {
      availability = 0.995
      latency      = 0.9
    }
    modeler = {
      availability = 0.995
      latency      = 0.9
    }
    server = {
      availability = 0.995
      latency      = 0.9
    }
  }
}

resource "google_monitoring_custom_service" "verification-server" {
  service_id   = "verification-server"
  display_name = "Verification Server"
  project      = var.project
}

module "availability-slos" {
  source = "./module.availability-slo"

  project               = var.project
  custom-service-id     = google_monitoring_custom_service.verification-server.service_id
  enabled               = var.https-forwarding-rule != ""
  notification-channels = google_monitoring_notification_channel.channels

  for_each = merge(local.default_slo_thresholds, var.slo_thresholds_overrides)

  service-name = each.key
  goal         = each.value.availability
}

module "latency-slos" {
  source = "./module.latency-slo"

  project = var.verification-server-project

  custom-service-id     = google_monitoring_custom_service.verification-server.service_id
  enabled               = var.https-forwarding-rule != ""
  notification-channels = google_monitoring_notification_channel.channels

  for_each = merge(local.default_slo_thresholds, var.slo_thresholds_overrides)

  service-name = each.key
  goal         = each.value.latency
}
