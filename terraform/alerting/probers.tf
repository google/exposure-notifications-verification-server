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

resource "google_monitoring_uptime_check_config" "https" {
  project = local.monitoring-host-project

  for_each = toset(compact(concat(var.server_hosts, var.apiserver_hosts, var.adminapi_hosts, var.extra-hosts)))

  display_name = each.key
  timeout      = "3s"
  period       = "60s"

  http_check {
    path         = "/health"
    port         = "443"
    use_ssl      = true
    validate_ssl = true
  }

  monitored_resource {
    type = "uptime_url"
    labels = {
      project_id = local.monitoring-host-project
      host       = each.key
    }
  }
  depends_on = [
    null_resource.manual-step-to-enable-workspace
  ]
}

resource "google_monitoring_alert_policy" "probers" {
  project = local.monitoring-host-project

  display_name = "Host Down"
  combiner     = "OR"
  conditions {
    display_name = "Host is unreachable"
    condition_threshold {
      duration        = "300s"
      threshold_value = 0.2
      comparison      = "COMPARISON_LT"
      filter          = "metric.type=\"monitoring.googleapis.com/uptime_check/check_passed\" resource.type=\"uptime_url\""

      aggregations {
        alignment_period     = "60s"
        cross_series_reducer = "REDUCE_FRACTION_TRUE"
        group_by_fields = [
          "resource.label.host",
        ]
        per_series_aligner = "ALIGN_NEXT_OLDER"
      }

      trigger {
        count = 1
      }
    }
  }

  documentation {
    content   = <<-EOT
## $${policy.display_name}

[$${resource.label.host}](https://$${resource.label.host}/health) is being reported unreachable.

See [docs/hosts.md](https://github.com/sethvargo/exposure-notifications-server-infra/blob/main/docs/hosts.md) for information about hosts.
EOT
    mime_type = "text/markdown"
  }

  notification_channels = [
    google_monitoring_notification_channel.email.id
  ]
  depends_on = [
    null_resource.manual-step-to-enable-workspace
  ]
}
