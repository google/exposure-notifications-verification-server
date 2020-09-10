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

resource "google_monitoring_dashboard" "verification-server" {
  project        = var.project
  dashboard_json = jsonencode(yamldecode(file("${path.module}/dashboards/verification-server.yaml")))
  depends_on = [
    null_resource.manual-step-to-enable-workspace
  ]
}

resource "google_monitoring_dashboard" "e2e" {
  project        = var.project
  dashboard_json = jsonencode(yamldecode(file("${path.module}/dashboards/e2e.yaml")))
  depends_on = [
    null_resource.manual-step-to-enable-workspace
  ]
}

resource "google_monitoring_alert_policy" "five_xx" {
  project      = var.project
  display_name = "Elevated 5xx"
  combiner     = "OR"
  conditions {
    display_name = "Elevated 5xx on Verification Server"
    condition_threshold {
      duration        = "300s"
      threshold_value = 0.2
      comparison      = "COMPARISON_GT"
      filter          = "metric.type=\"run.googleapis.com/request_count\" resource.type=\"cloud_run_revision\" metric.label.\"response_code_class\"=\"5xx\" resource.label.\"service_name\"!=\"e2e-runner\""

      aggregations {
        alignment_period     = "60s"
        cross_series_reducer = "REDUCE_SUM"
        group_by_fields = [
          "resource.label.service_name",
        ]
        per_series_aligner = "ALIGN_RATE"
      }

      trigger {
        count = 1
      }
    }
  }

  documentation {
    content   = <<-EOT
## $${policy.display_name}

[$${resource.label.host}](https://$${resource.label.host}/) is reporting elevated 5xx errors.

See [docs/5xx.md](https://github.com/sethvargo/exposure-notifications-server-infra/blob/main/docs/5xx.md) for information about debugging.
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
