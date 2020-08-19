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

# TODO(icco): This is currently all setup manually.
#resource "google_compute_backend_service" "apiserver" {
#  provider   = google-beta
#  name       = "apiserver"
#  project    = var.project
#  enable_cdn = true
#
#  backend {
#    group = google_compute_region_network_endpoint_group.apiserver.id
#  }
#}

resource "google_compute_global_address" "verification-server" {
  name    = "verification-server-address"
  project = var.project
}

#resource "google_compute_url_map" "urlmap" {
#  name            = "verification-server"
#  project         = var.project
#  default_service = google_compute_backend_service.apiserver.id
#
# TODO(icco): Add host base routing for all four services.
#}
#
#resource "google_compute_target_http_proxy" "default" {
#  name    = "verification-server"
#  project = var.project
#  url_map = google_compute_url_map.urlmap.id
#}
#
#resource "google_compute_forwarding_rule" "verification-server" {
#  provider = google-beta
#  name     = "verification-server"
#  project  = var.project
#
#  ip_protocol           = "TCP"
#  ip_address            = google_compute_global_address.verification-server.address
#  load_balancing_scheme = "EXTERNAL"
#  port_range            = "80"
#  target                = google_compute_target_http_proxy.default.id
#  network_tier          = "PREMIUM"
#}
