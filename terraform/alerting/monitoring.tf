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
      threshold_value = 2
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

resource "google_monitoring_alert_policy" "rate_limited_count" {
  project      = var.project
  display_name = "ElevatedRateLimitedCount"
  combiner     = "OR"
  conditions {
    display_name = "/rate_limited_count"
    condition_threshold {
      duration        = "300s"
      threshold_value = 1
      comparison      = "COMPARISON_GT"
      filter          = "metric.type=\"custom.googleapis.com/opencensus/en-verification-server/ratelimit/limitware/rate_limited_count\" resource.type=\"generic_task\""

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

[$${resource.label.host}](https://$${resource.label.host}) request
throttled by ratelimit middleware. This could indicate a bad behaving
client app, or a potential DoS attack.

View the metric here

https://console.cloud.google.com/monitoring/dashboards/custom/${basename(google_monitoring_dashboard.verification-server.id)}?project=${var.project}
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

resource "google_monitoring_alert_policy" "backend_latency" {
  project      = var.project
  display_name = "Elevated Latency Greater than 2s"
  combiner     = "OR"
  conditions {
    display_name = "/backend_latencies"
    condition_threshold {
      duration        = "300s"
      threshold_value = "2000"
      comparison      = "COMPARISON_GT"
      filter          = "metric.type=\"loadbalancing.googleapis.com/https/backend_latencies\" resource.type=\"https_lb_rule\" "

      aggregations {
        alignment_period     = "60s"
        cross_series_reducer = "REDUCE_PERCENTILE_95"
        group_by_fields = [
          "resource.label.backend_target_name",
        ]
        per_series_aligner = "ALIGN_DELTA"
      }

      trigger {
        count = 1
      }
    }
  }

  documentation {
    content   = <<-EOT
## $${policy.display_name}

[$${resource.label.host}](https://$${resource.label.host}) Latency is spiking in the server

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

resource "google_monitoring_alert_policy" "realm_token_capacity" {
  project      = var.project
  display_name = "RealmTokenCapacityUtilizationAboveThreshold"
  combiner     = "OR"
  conditions {
    display_name = "/realm_capacity_latest"
    condition_threshold {
      duration        = "300s"
      threshold_value = 0.9
      comparison      = "COMPARISON_GT"
      filter          = "metric.type=\"custom.googleapis.com/opencensus/en-verification-server/api/issue/realm_token_capacity_latest\" resource.type=\"generic_task\""

      aggregations {
        alignment_period     = "60s"
        group_by_fields = [
          "resource.label.realm",
        ]
        per_series_aligner = "ALIGN_MAX"
      }

      trigger {
        count = 1
      }
    }
  }

  documentation {
    content   = <<-EOT
## $${policy.display_name}

[$${resource.label.realm}](https://$${resource.label.realm}) realm
daily verification code issuing capacity utilized above 90%.

View the metric here

https://console.cloud.google.com/monitoring/dashboards/custom/${basename(google_monitoring_dashboard.verification-server.id)}?project=${var.project}
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
