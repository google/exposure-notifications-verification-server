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

resource "null_resource" "RealmTokenRemainingCapacityLowAlert" {
  triggers = {
    # trigger a provision if the content changes.
    file_content = file("${path.module}/alerts/RealmTokenRemainingCapacityLow.yaml"),
  }
  provisioner "local-exec" {
    command = "${path.module}/scripts/upsert_alert_policy.sh"
    environment = {
      CLOUDSDK_CORE_PROJECT = var.monitoring-host-project
      POLICY                = self.triggers.file_content
      DISPLAY_NAME          = "RealmTokenRemainingCapacityLow"
    }
  }
  depends_on = [
    google_monitoring_metric_descriptor.api--issue--realm_token_latest,
  ]
}

resource "null_resource" "E2ETestErrorRatioHigh" {
  triggers = {
    # trigger a provision if the content changes.
    file_content = file("${path.module}/alerts/E2ETestErrorRatioHigh.yaml"),
  }
  provisioner "local-exec" {
    command = "${path.module}/scripts/upsert_alert_policy.sh"
    environment = {
      CLOUDSDK_CORE_PROJECT = var.monitoring-host-project
      POLICY                = self.triggers.file_content
      DISPLAY_NAME          = "E2ETestErrorRatioHigh"
    }
  }
  depends_on = [
    google_monitoring_metric_descriptor.e2e--request_count
  ]
}
