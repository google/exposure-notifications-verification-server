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
  playbook_prefix = "https://github.com/google/exposure-notifications-verification-server/blob/main/docs/playbooks/slo"
}

resource "google_monitoring_slo" "slo" {
  # the basics
  service      = var.custom_service_id
  slo_id       = "availability-slo-${var.service_name}"
  display_name = "${var.goal * 100}% of requests are successful over rolling 28 days (service=${var.service_name})"
  project      = var.project
  count        = var.enabled ? 1 : 0


  # the SLI
  request_based_sli {
    good_total_ratio {
      good_service_filter = <<-EOT
        metric.type="loadbalancing.googleapis.com/https/request_count"
        resource.type="https_lb_rule"
        resource.label.backend_name="${var.service_name}"
        metric.label.response_code_class=200
      EOT
      bad_service_filter  = <<-EOT
        metric.type="loadbalancing.googleapis.com/https/request_count"
        resource.type="https_lb_rule"
        resource.label.backend_name="${var.service_name}"
        metric.label.response_code_class=500
      EOT
    }
  }

  # the goal
  goal                = var.goal
  rolling_period_days = 28
}

# fast error budget burn alert
resource "google_monitoring_alert_policy" "fast_burn" {
  project      = var.project
  display_name = "FastErrorBudgetBurn-${var.service_name}"
  combiner     = "AND"
  enabled      = var.enable_alert
  count        = var.enabled ? 1 : 0

  conditions {
    display_name = "Fast burn over last hour"
    condition_threshold {
      filter     = <<-EOT
      select_slo_burn_rate("${google_monitoring_slo.slo.id}", "1h")
      EOT
      duration   = "0s"
      comparison = "COMPARISON_GT"
      # burn rate = budget consumed * period / alerting window = .02 * (7 * 24 * 60)/60 = 3.36
      threshold_value = 3.36
      trigger {
        count = 1
      }
    }
  }

  conditions {
    display_name = "Fast burn over last 5 minutes"
    condition_threshold {
      filter     = <<-EOT
      select_slo_burn_rate("${google_monitoring_slo.slo.id}", "5m")
      EOT
      duration   = "0s"
      comparison = "COMPARISON_GT"
      # burn rate = budget consumed * period / alerting window = .02 * (7 * 24 * 60)/60 = 3.36
      threshold_value = 3.36
      trigger {
        count = 1
      }
    }
  }

  documentation {
    content   = "${local.playbook_prefix}/FastErrorBudgetBurn.md"
    mime_type = "text/markdown"
  }

  notification_channels = [for x in values(var.notification_channels) : x.id]

  depends_on = [
    google_monitoring_slo.slo,
  ]
}

# slow error budget burn alert
resource "google_monitoring_alert_policy" "slow_burn" {
  project      = var.project
  count        = var.enabled ? 1 : 0
  display_name = "SlowErrorBudgetBurn-${var.service_name}"
  combiner     = "AND"
  enabled      = var.enable_alert

  conditions {
    display_name = "Slow burn over last 6 hours"
    condition_threshold {
      filter     = <<-EOT
      select_slo_burn_rate("${google_monitoring_slo.slo.id}", "6h")
      EOT
      duration   = "0s"
      comparison = "COMPARISON_GT"
      # burn rate = budget consumed * period / alerting window = .05 * (7 * 24 * 60)/360 = 1.4
      threshold_value = 1.4
      trigger {
        count = 1
      }
    }
  }

  conditions {
    display_name = "Slow burn over last 30 minutes"
    condition_threshold {
      filter     = <<-EOT
      select_slo_burn_rate("${google_monitoring_slo.slo.id}", "30m")
      EOT
      duration   = "0s"
      comparison = "COMPARISON_GT"
      # burn rate = budget consumed * period / alerting window = .05 * (7 * 24 * 60)/360 = 1.4
      threshold_value = 1.4
      trigger {
        count = 1
      }
    }
  }

  documentation {
    content   = "${local.playbook_prefix}/SlowErrorBudgetBurn.md"
    mime_type = "text/markdown"
  }

  notification_channels = [for x in values(var.notification_channels) : x.id]

  depends_on = [
    google_monitoring_slo.slo,
  ]
}
