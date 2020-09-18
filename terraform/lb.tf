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

locals {
  enable_lb       = var.server-host != "" && var.apiserver-host != "" && var.adminapi-host != ""
  enable_redirect = length(var.redirect_domain_map) > 0
}

resource "google_compute_global_address" "verification-server" {
  count   = local.enable_lb ? 1 : 0
  name    = "verification-server-address"
  project = var.project
}

resource "google_compute_global_address" "verification-redirect" {
  count   = local.enable_redirect ? 1 : 0
  name    = "verification-redirect-address"
  project = var.project
}

# Redirects all requests to https
resource "google_compute_url_map" "urlmap-http" {
  name     = "https-redirect"
  provider = google-beta
  project  = var.project

  default_url_redirect {
    strip_query    = false
    https_redirect = true
  }
}

resource "google_compute_url_map" "urlmap-https" {
  count           = local.enable_lb ? 1 : 0
  name            = "verification-server"
  provider        = google-beta
  project         = var.project
  default_service = google_compute_backend_service.server[0].id

  host_rule {
    hosts        = [var.server-host]
    path_matcher = "server"
  }

  path_matcher {
    name            = "server"
    default_service = google_compute_backend_service.server[0].id
  }

  host_rule {
    hosts        = [var.apiserver-host]
    path_matcher = "apiserver"
  }

  path_matcher {
    name            = "apiserver"
    default_service = google_compute_backend_service.apiserver[0].id
  }

  host_rule {
    hosts        = [var.adminapi-host]
    path_matcher = "adminapi"
  }

  path_matcher {
    name            = "adminapi"
    default_service = google_compute_backend_service.adminapi[0].id
  }
}

resource "google_compute_url_map" "redirect-urlmap-https" {
  count           = local.enable_redirect ? 1 : 0
  name            = "verification-redirect"
  provider        = google-beta
  project         = var.project
  default_service = google_compute_backend_service.redirect[0].id
}

resource "google_compute_target_http_proxy" "http" {
  count    = local.enable_lb ? 1 : 0
  provider = google-beta
  name     = "verification-server"
  project  = var.project

  url_map = google_compute_url_map.urlmap-http.id
}

resource "google_compute_target_https_proxy" "https" {
  count   = local.enable_lb ? 1 : 0
  name    = "verification-server"
  project = var.project

  url_map          = google_compute_url_map.urlmap-https[0].id
  ssl_certificates = [google_compute_managed_ssl_certificate.default[0].id]
}

resource "google_compute_target_http_proxy" "redirect-http" {
  count    = local.enable_redirect ? 1 : 0
  provider = google-beta
  name     = "verification-redirect"
  project  = var.project

  url_map = google_compute_url_map.urlmap-http.id
}

resource "google_compute_target_https_proxy" "redirect-https" {
  count   = local.enable_redirect ? 1 : 0
  name    = "verification-redirect"
  project = var.project

  url_map          = google_compute_url_map.urlmap-https[0].id
  ssl_certificates = [google_compute_managed_ssl_certificate.redirect[0].id]
}

resource "google_compute_global_forwarding_rule" "http" {
  count    = local.enable_lb ? 1 : 0
  provider = google-beta
  name     = "verification-server-http"
  project  = var.project

  ip_protocol           = "TCP"
  ip_address            = google_compute_global_address.verification-server[0].address
  load_balancing_scheme = "EXTERNAL"
  port_range            = "80"
  target                = google_compute_target_http_proxy.http[0].id
}

resource "google_compute_global_forwarding_rule" "https" {
  count    = local.enable_lb ? 1 : 0
  provider = google-beta
  name     = "verification-server-https"
  project  = var.project

  ip_protocol           = "TCP"
  ip_address            = google_compute_global_address.verification-server[0].address
  load_balancing_scheme = "EXTERNAL"
  port_range            = "443"
  target                = google_compute_target_https_proxy.https[0].id
}

resource "google_compute_global_forwarding_rule" "redirect-http" {
  count    = local.enable_redirect ? 1 : 0
  provider = google-beta
  name     = "verification-redirect-http"
  project  = var.project

  ip_protocol           = "TCP"
  ip_address            = google_compute_global_address.verification-redirect[0].address
  load_balancing_scheme = "EXTERNAL"
  port_range            = "80"
  target                = google_compute_target_http_proxy.http[0].id
}

resource "google_compute_global_forwarding_rule" "redirect-https" {
  count    = local.enable_redirect ? 1 : 0
  provider = google-beta
  name     = "verification-redirect-https"
  project  = var.project

  ip_protocol           = "TCP"
  ip_address            = google_compute_global_address.verification-redirect[0].address
  load_balancing_scheme = "EXTERNAL"
  port_range            = "443"
  target                = google_compute_target_https_proxy.https[0].id
}

resource "google_compute_managed_ssl_certificate" "default" {
  count    = local.enable_lb ? 1 : 0
  provider = google-beta

  name = "verification-cert"

  managed {
    domains = [var.server-host, var.apiserver-host, var.adminapi-host]
  }
}

resource "google_compute_managed_ssl_certificate" "redirect" {
  count    = local.enable_redirect ? 1 : 0
  provider = google-beta

  name = "verification-redirect-cert"

  managed {
    // we can only have 100 domains in this list.
    domains = [for o in var.redirect_domain_map : o.host]
  }
}
