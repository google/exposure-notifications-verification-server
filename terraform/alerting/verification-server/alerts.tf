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

resource "google_monitoring_alert_policy" "RealmTokenRemainingCapacityLow" {
  project      = var.verification-server-project
  combiner     = "OR"
  display_name = "RealmTokenRemainingCapacityLow"
  conditions {
    display_name = "Per-realm issue API token remaining capacity"
    condition_monitoring_query_language {
      duration = "600s"
      query    = <<-EOT
      fetch
      generic_task :: custom.googleapis.com/opencensus/en-verification-server/api/issue/realm_token_latest
      | {
        AVAILABLE: filter metric.state == 'AVAILABLE' | align
        ;
        LIMIT: filter metric.state == 'LIMIT' | align
      }
      | group_by [metric.realm], [val: sum(value.realm_token_latest)]
      | ratio
      | window 1m
      | condition ratio < 0.1
      EOT
      trigger {
        count = 1
      }
    }
  }
  documentation {
    content   = <<-EOT
    ## $${policy.display_name}

    Realm $${metric.label.realm} daily verification code issuing remaining capacity below 10%.
    EOT
    mime_type = "text/markdown"
  }
  notification_channels = [
    google_monitoring_notification_channel.email.id
  ]
  depends_on = [
    google_monitoring_metric_descriptor.api--issue--realm_token_latest,
  ]
}

resource "null_resource" "E2ETestErrorRatioHigh" {
  triggers = {
    # trigger a provision if the content changes.
    file_content = file("${path.module}/alerts/E2ETestErrorRatioHigh.yaml"),

    notification_channel = google_monitoring_notification_channel.email.id
  }
  provisioner "local-exec" {
    command = "${path.module}/scripts/upsert_alert_policy.sh"
    environment = {
      CLOUDSDK_CORE_PROJECT = var.monitoring-host-project
      POLICY                = self.triggers.file_content
      DISPLAY_NAME          = "E2ETestErrorRatioHigh"
      NOTIFICATION_CHANNEL  = self.triggers.notification_channel
    }
  }
  depends_on = [
    google_monitoring_metric_descriptor.e2e--request_count
  ]
}
