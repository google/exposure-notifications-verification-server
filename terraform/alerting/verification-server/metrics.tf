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

resource "google_monitoring_metric_descriptor" "api--issue--request_count" {
  project      = var.verification-server-project
  description  = "Count of code issue requests"
  display_name = "OpenCensus/en-verification-server/api/issue/request_count"
  type         = "custom.googleapis.com/opencensus/en-verification-server/api/issue/request_count"
  metric_kind  = "CUMULATIVE"
  value_type   = "INT64"
  unit         = "1"
  labels { key = "realm" }
  labels { key = "build_id" }
  labels { key = "build_tag" }
  labels { key = "blame" }
  labels { key = "result" }
}
resource "google_monitoring_metric_descriptor" "api--verify--request_count" {
  project      = var.verification-server-project
  description  = "Count of verify requests"
  display_name = "OpenCensus/en-verification-server/api/verify/request_count"
  type         = "custom.googleapis.com/opencensus/en-verification-server/api/verify/request_count"
  metric_kind  = "CUMULATIVE"
  value_type   = "INT64"
  unit         = "1"
  labels { key = "realm" }
  labels { key = "build_id" }
  labels { key = "build_tag" }
  labels { key = "blame" }
  labels { key = "result" }
}

resource "google_monitoring_metric_descriptor" "api--issue--realm_token_latest" {
  project      = var.verification-server-project
  description  = "Latest realm token count"
  display_name = "OpenCensus/en-verification-server/api/issue/realm_token_latest"
  type         = "custom.googleapis.com/opencensus/en-verification-server/api/issue/realm_token_latest"
  metric_kind  = "GAUGE"
  value_type   = "INT64"
  unit         = "1"
  labels { key = "realm" }
  labels { key = "build_id" }
  labels { key = "build_tag" }
  labels { key = "state" }
}

resource "google_monitoring_metric_descriptor" "ratelimit--limitware--request_count" {
  project      = var.verification-server-project
  description  = "requests seen by middleware"
  display_name = "OpenCensus/en-verification-server/ratelimit/limitware/request_count"
  type         = "custom.googleapis.com/opencensus/en-verification-server/ratelimit/limitware/request_count"
  metric_kind  = "CUMULATIVE"
  value_type   = "INT64"
  unit         = "1"
  labels { key = "realm" }
  labels { key = "build_id" }
  labels { key = "build_tag" }
  labels { key = "result" }
}

resource "google_monitoring_metric_descriptor" "e2e--request_count" {
  project      = var.verification-server-project
  description  = "Count of e2e requests"
  display_name = "OpenCensus/en-verification-server/e2e/request_count"
  type         = "custom.googleapis.com/opencensus/en-verification-server/e2e/request_count"
  metric_kind  = "CUMULATIVE"
  value_type   = "INT64"
  unit         = "1"
  labels { key = "realm" }
  labels { key = "build_id" }
  labels { key = "build_tag" }
  labels { key = "test_type" }
  labels { key = "result" }
  labels { key = "step" }
}
