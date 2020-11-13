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

resource "google_monitoring_custom_service" "verification-server" {
  service_id = "verification-server"
  display_name = "Verification Server"
  project = var.monitoring-host-project
}

resource "google_monitoring_slo" "availability-slo" {
  # the basics
  service = google_monitoring_custom_service.verification-server.service_id
  slo_id = "availability-slo"
  display_name = "99.9% of requests are successful over rolling 28 days"

  # the SLI
  request_based_sli {
    good_total_ratio {
      good_service_filter = join(" AND ", [
        "metric.type=\"loadbalancing.googleapis.com/https/request_count\"",
        "resource.type=\"https_lb_rule\"",
        "resource.label.\"backend_name\"=\"apiserver\"",
        "metric.label.\"response_code_class\"=\"200\""
      ])
      bad_service_filter = join(" AND ", [
        "metric.type=\"loadbalancing.googleapis.com/https/request_count\"",
        "resource.type=\"https_lb_rule\"",
        "resource.label.\"backend_name\"=\"apiserver\"",
        "metric.label.\"response_code_class\"=\"500\""
    }
  }

  # the goal
  goal = 0.999
  rolling_period_days = 28
}