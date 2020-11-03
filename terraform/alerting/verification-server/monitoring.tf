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
  project        = var.monitoring-host-project
  dashboard_json = jsonencode(yamldecode(file("${path.module}/dashboards/verification-server.yaml")))
  depends_on = [
    null_resource.manual-step-to-enable-workspace,
    google_monitoring_metric_descriptor.api--issue--request_count,
    google_monitoring_metric_descriptor.api--issue--realm_token_latest,
    google_monitoring_metric_descriptor.ratelimit--limitware--request_count
  ]
}

resource "google_monitoring_dashboard" "e2e" {
  project        = var.monitoring-host-project
  dashboard_json = jsonencode(yamldecode(file("${path.module}/dashboards/e2e.yaml")))
  depends_on = [
    null_resource.manual-step-to-enable-workspace,
    google_monitoring_metric_descriptor.api--issue--request_count,
    google_monitoring_metric_descriptor.api--verify--request_count,
  ]
}

resource "google_monitoring_alert_policy" "five_xx" {
  project      = var.monitoring-host-project
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
  project      = var.monitoring-host-project
  display_name = "Elevated Rate Limited Count"
  combiner     = "OR"
  conditions {
    display_name = "Rate Limited count by service_name"
    condition_threshold {
      duration        = "300s"
      threshold_value = 1
      comparison      = "COMPARISON_GT"
      filter          = <<-EOT
      metric.type="custom.googleapis.com/opencensus/en-verification-server/ratelimit/limitware/request_count"
      resource.type="generic_task"
      metric.label.result="RATE_LIMITED"
EOT

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

https://console.cloud.google.com/monitoring/dashboards/custom/${basename(google_monitoring_dashboard.verification-server.id)}?project=${var.monitoring-host-project}
EOT
    mime_type = "text/markdown"
  }

  notification_channels = [
    google_monitoring_notification_channel.email.id
  ]
  depends_on = [
    null_resource.manual-step-to-enable-workspace,
    google_monitoring_metric_descriptor.ratelimit--limitware--request_count,
  ]
}

resource "google_monitoring_alert_policy" "backend_latency" {
  project      = var.verification-server-project
  display_name = "Elevated Latency Greater than 2s"
  combiner     = "OR"
  conditions {
    display_name = "/backend_latencies"
    condition_threshold {
      duration        = "300s"
      threshold_value = "2000"
      comparison      = "COMPARISON_GT"
      filter          = "metric.type=\"loadbalancing.googleapis.com/https/backend_latencies\" resource.type=\"https_lb_rule\" resource.label.\"backend_name\"!=\"NO_BACKEND_SELECTED\" resource.label.\"forwarding_rule_name\"=\"verification-server-https\""

      aggregations {
        alignment_period     = "60s"
        cross_series_reducer = "REDUCE_PERCENTILE_99"
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

Latency has been above 2s for > 5 minutes on $${resource.label.backend_target_name}.

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

resource "google_logging_metric" "requests_by_host" {
  name    = "requests_by_host"
  project = var.verification-server-project

  filter = <<-EOT
resource.type=cloud_run_revision 
httpRequest.requestUrl!=""
EOT

  metric_descriptor {
    metric_kind = "DELTA"
    value_type  = "INT64"
    labels {
      key        = "host"
      value_type = "STRING"
    }
  }

  label_extractors = {
    "host" = "REGEXP_EXTRACT(httpRequest.requestUrl, \"^https?://([a-z0-9\\\\-._~%]+|\\\\[[a-z0-9\\\\-._~%!$&'()*+,;=:]+\\\\])/.*$\")"
  }
}
