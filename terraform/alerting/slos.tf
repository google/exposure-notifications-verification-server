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

locals {
  default_per_service_slo = {
    enable_fast_burn_alert  = false
    availability_goal       = 0.995
    enable_availability_slo = false
    latency_threshold       = 60000 # 60 seconds, in ms
    enable_latency_alert    = false
    latency_alert_duration  = 300000 # 5 minutes, in ms

  }
  service_configs = {
    adminapi = merge(local.default_per_service_slo,
      { enable_latency_alert = true
    latency_threshold = 6000 })
    apiserver = merge(local.default_per_service_slo,
      { enable_availability_slo = true,
    enable_fast_burn_alert = true })
    appsync      = local.default_per_service_slo
    cleanup      = local.default_per_service_slo
    e2e-runner   = local.default_per_service_slo
    enx-redirect = local.default_per_service_slo
    modeler      = local.default_per_service_slo
    rotation     = local.default_per_service_slo
    server = merge(local.default_per_service_slo,
      { enable_latency_alert = true,
    latency_threshold = 2000 })
    stats-puller = local.default_per_service_slo
  }
}

module "services" {
  source = "./module.service"

  project = var.project

  for_each          = merge(local.service_configs, var.slo_thresholds_overrides)
  service_name      = each.key
  display_name      = each.key
  latency_threshold = each.value.latency_threshold
}

module "latency-alerts" {
  source = "./module.latency-alert"

  notification_channels = google_monitoring_notification_channel.paging

  project           = var.project
  for_each          = merge(local.service_configs, var.slo_thresholds_overrides)
  custom_service_id = each.key
  enabled           = each.value.enable_latency_alert
  service_name      = each.key
  threshold         = each.value.latency_threshold
  duration          = each.value.latency_alert_duration
}

module "availability-slos" {
  source = "./module.availability-slo"

  project               = var.project
  notification_channels = google_monitoring_notification_channel.paging

  for_each = merge(local.service_configs, var.slo_thresholds_overrides)

  enabled                = each.value.enable_availability_slo
  custom_service_id      = each.key
  service_name           = each.key
  goal                   = each.value.availability_goal
  enable_fast_burn_alert = each.value.enable_fast_burn_alert
}
