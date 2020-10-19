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
  enable_enx_redirect = length(var.enx_redirect_domain_map) > 0
}

resource "google_compute_global_address" "verification-enx-redirect" {
  count   = local.enable_enx_redirect ? 1 : 0
  name    = "verification-enx-redirect-address"
  project = var.project
}

# Redirects all requests to https
resource "google_compute_url_map" "enx-redirect-urlmap-http" {
  name     = "enx-redirect-https-redirect"
  provider = google-beta
  project  = var.project

  default_url_redirect {
    strip_query    = false
    https_redirect = true
  }
}

resource "google_compute_url_map" "enx-redirect-urlmap-https" {
  count           = local.enable_enx_redirect ? 1 : 0
  name            = "verification-enx-redirect"
  provider        = google-beta
  project         = var.project
  default_service = google_compute_backend_service.enx-redirect[0].id
}

resource "google_compute_target_http_proxy" "enx-redirect-http" {
  count    = local.enable_enx_redirect ? 1 : 0
  provider = google-beta
  name     = "verification-enx-redirect"
  project  = var.project

  url_map = google_compute_url_map.enx-redirect-urlmap-http.id
}

resource "google_compute_target_https_proxy" "enx-redirect-https" {
  count   = local.enable_enx_redirect ? 1 : 0
  name    = "verification-enx-redirect"
  project = var.project

  url_map = google_compute_url_map.enx-redirect-urlmap-https[0].id
  ssl_certificates = [
    // First certificate is harder to change in UI, so let's keep a separate
    // unused cert in the first slot.
    google_compute_managed_ssl_certificate.enx-redirect-root[0].id,
    google_compute_managed_ssl_certificate.enx-redirect[0].id
  ]

  # Defined in verification-lb.tf
  ssl_policy = google_compute_ssl_policy.one-two-ssl-policy.id
}

resource "google_compute_global_forwarding_rule" "enx-redirect-http" {
  count    = local.enable_enx_redirect ? 1 : 0
  provider = google-beta
  name     = "verification-enx-redirect-http"
  project  = var.project

  ip_protocol           = "TCP"
  ip_address            = google_compute_global_address.verification-enx-redirect[0].address
  load_balancing_scheme = "EXTERNAL"
  port_range            = "80"
  target                = google_compute_target_http_proxy.enx-redirect-http[0].id
}

resource "google_compute_global_forwarding_rule" "enx-redirect-https" {
  count    = local.enable_enx_redirect ? 1 : 0
  provider = google-beta
  name     = "verification-enx-redirect-https"
  project  = var.project

  ip_protocol           = "TCP"
  ip_address            = google_compute_global_address.verification-enx-redirect[0].address
  load_balancing_scheme = "EXTERNAL"
  port_range            = "443"
  target                = google_compute_target_https_proxy.enx-redirect-https[0].id
}

resource "google_compute_managed_ssl_certificate" "enx-redirect-root" {
  count    = local.enable_enx_redirect ? 1 : 0
  provider = google-beta

  name        = "verification-enx-redirect-cert-root"
  description = "Controlled by Terraform"

  managed {
    domains = ["www.${var.enx_redirect_domain}"]
  }
}

resource "google_compute_managed_ssl_certificate" "enx-redirect" {
  count    = local.enable_enx_redirect ? 1 : 0
  provider = google-beta

  name        = "verification-enx-redirect-cert"
  description = "Controlled by Terraform"

  managed {
    // we can only have 100 domains in this list.
    domains = [for o in var.enx_redirect_domain_map : o.host]
  }
}
