# Copyright 2021 the Exposure Notifications Verification Server authors
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

# latency alert
# alert if mean latency is over $threshold for $duration
resource "google_monitoring_alert_policy" "latency-alert" {
  count        = var.enabled ? 1 : 0
  project      = var.project
  display_name = "LatencyOverThreshold-${var.service_name}"
  combiner     = "AND"
  enabled      = var.enabled

  conditions {
    display_name = "Latency over ${var.threshold / 1000} seconds for ${var.duration / 1000 / 60} minutes"
    condition_monitoring_query_language {
      duration = "${var.duration / 1000}s"
      query    = <<-EOT
        fetch https_lb_rule
        | metric 'loadbalancing.googleapis.com/https/total_latencies'
        | filter (resource.backend_name == '${var.service_name}')
        | group_by 1m,
            [value_total_latencies_percentile: percentile(value.total_latencies, 99)]
        | every 1m
        | group_by [],
            [val:
            mean(value_total_latencies_percentile)]
        | condition val > ${var.threshold} 'ms'
      EOT
      trigger {
        count = 1
      }
    }
  }

  documentation {
    content   = "${local.playbook_prefix}/FastLatencyBudgetBurn.md"
    mime_type = "text/markdown"
  }

  notification_channels = [for x in values(var.notification_channels) : x.id]

}
