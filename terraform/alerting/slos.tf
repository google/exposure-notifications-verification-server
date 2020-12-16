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
  default_per_service_slo = {
    availability_goal = 0.995
    latency = {
      goal      = 0.9
      threshold = 60000
    }
    enable_alert = false
  }
  default_slo_thresholds = {
    adminapi     = merge(local.default_per_service_slo, { enable_alert = true })
    apiserver    = merge(local.default_per_service_slo, { enable_alert = true })
    appsync      = local.default_per_service_slo
    cleanup      = local.default_per_service_slo
    e2e-runner   = local.default_per_service_slo
    enx-redirect = local.default_per_service_slo
    modeler      = local.default_per_service_slo
    server       = merge(local.default_per_service_slo, { enable_alert = true })
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
  custom_service_id     = google_monitoring_custom_service.verification-server.service_id
  enabled               = var.https-forwarding-rule != ""
  notification_channels = google_monitoring_notification_channel.channels

  for_each = merge(local.default_slo_thresholds, var.slo_thresholds_overrides)

  service_name = each.key
  goal         = each.value.availability_goal
  enable_alert = each.value.enable_alert
}

module "latency-slos" {
  source = "./module.latency-slo"

  project = var.project

  custom_service_id     = google_monitoring_custom_service.verification-server.service_id
  enabled               = var.https-forwarding-rule != ""
  notification_channels = google_monitoring_notification_channel.channels

  for_each = merge(local.default_slo_thresholds, var.slo_thresholds_overrides)

  service_name = each.key
  goal         = each.value.latency.goal
  threshold    = each.value.latency.threshold
  enable_alert = each.value.enable_alert
}
