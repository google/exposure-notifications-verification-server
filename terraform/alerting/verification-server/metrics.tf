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

resource "google_monitoring_metric_descriptor" "ratelimit--limitware--rate_limited_count" {
  project      = var.verification-server-project
  description  = "rate limited requests"
  display_name = "OpenCensus/en-verification-server/ratelimit/limitware/rate_limited_count"
  type         = "custom.googleapis.com/opencensus/en-verification-server/ratelimit/limitware/rate_limited_count"
  metric_kind  = "CUMULATIVE"
  value_type   = "INT64"
  unit         = "1"
  labels {
    key        = "realm"
    value_type = "STRING"
  }
  labels {
    key        = "build_id"
    value_type = "STRING"
  }
  labels {
    key        = "build_tag"
    value_type = "STRING"
  }
}

resource "google_monitoring_metric_descriptor" "api--issue--realm_token_capacity_latest" {
  project      = var.verification-server-project
  description  = "Latest realm token capacity utilization"
  display_name = "OpenCensus/en-verification-server/api/issue/realm_token_capacity_latest"
  type         = "custom.googleapis.com/opencensus/en-verification-server/api/issue/realm_token_capacity_latest"
  metric_kind  = "GAUGE"
  value_type   = "DOUBLE"
  unit         = "1"
  labels {
    key        = "realm"
    value_type = "STRING"
  }
  labels {
    key        = "build_id"
    value_type = "STRING"
  }
  labels {
    key        = "build_tag"
    value_type = "STRING"
  }
}

resource "google_monitoring_metric_descriptor" "api--issue--request_count" {
  project      = var.verification-server-project
  description  = "Count of code issue requests"
  display_name = "OpenCensus/en-verification-server/api/issue/request_count"
  type         = "custom.googleapis.com/opencensus/en-verification-server/api/issue/request_count"
  metric_kind  = "CUMULATIVE"
  value_type   = "INT64"
  unit         = "1"
  labels {
    key        = "realm"
    value_type = "STRING"
  }
  labels {
    key        = "build_id"
    value_type = "STRING"
  }
  labels {
    key        = "build_tag"
    value_type = "STRING"
  }
  labels {
    key        = "blame"
    value_type = "STRING"
  }
  labels {
    key        = "result"
    value_type = "STRING"
  }
}
resource "google_monitoring_metric_descriptor" "api--verify--request_count" {
  project      = var.verification-server-project
  description  = "Count of verify requests"
  display_name = "OpenCensus/en-verification-server/api/verify/request_count"
  type         = "custom.googleapis.com/opencensus/en-verification-server/api/verify/request_count"
  metric_kind  = "CUMULATIVE"
  value_type   = "INT64"
  unit         = "1"
  labels {
    key        = "realm"
    value_type = "STRING"
  }
  labels {
    key        = "build_id"
    value_type = "STRING"
  }
  labels {
    key        = "build_tag"
    value_type = "STRING"
  }
  labels {
    key        = "blame"
    value_type = "STRING"
  }
  labels {
    key        = "result"
    value_type = "STRING"
  }
}

