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
  ]
}

resource "google_monitoring_dashboard" "e2e" {
  project        = var.monitoring-host-project
  dashboard_json = jsonencode(yamldecode(file("${path.module}/dashboards/e2e.yaml")))
  depends_on = [
    null_resource.manual-step-to-enable-workspace,
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

resource "google_logging_metric" "stackdriver_export_error_count" {
  project     = var.verification-server-project
  name        = "stackdriver_export_error_count"
  description = "Error occurred trying to export metrics to stackdriver"

  filter = <<-EOT
  resource.type="cloud_run_revision"
  jsonPayload.logger="stackdriver"
  jsonPayload.message="failed to export metric"
  EOT

  metric_descriptor {
    metric_kind = "DELTA"
    unit        = "1"
    value_type  = "INT64"
  }
}
