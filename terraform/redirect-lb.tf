# Copyright 2020 the Exposure Notifications Verification Server authors
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

  redirect_root_domains = distinct(compact([
    var.enx_redirect_domain,
  ]))

  enx_domains = [for o in concat(var.enx_redirect_domain_map, var.enx_redirect_domain_map_add) : o.host]
}

resource "google_compute_global_address" "verification-enx-redirect" {
  count   = local.enable_enx_redirect ? 1 : 0
  name    = "verification-enx-redirect-address"
  project = var.project

  depends_on = [
    google_project_service.services["compute.googleapis.com"],
  ]
}

# Redirects all requests to https
resource "google_compute_url_map" "enx-redirect-urlmap-http" {
  name     = "enx-redirect-https-redirect"
  provider = google-beta
  project  = var.project

  host_rule {
    hosts        = toset(formatlist("*.%s", local.redirect_root_domains))
    path_matcher = "enx-http-https-redirect"
  }

  path_matcher {
    name = "enx-http-https-redirect"

    default_url_redirect {
      https_redirect = true
      strip_query    = false

      // Use a 302 response code in case we ever need to change the redirect.
      redirect_response_code = "FOUND"
    }
  }

  default_url_redirect {
    host_redirect  = "g.co"
    path_redirect  = "/ens"
    strip_query    = false
    https_redirect = true

    // Use a 302 response code. We want to avoid a client caching a domain that
    // may actually exist in the future.
    redirect_response_code = "FOUND"
  }

  depends_on = [
    google_project_service.services["compute.googleapis.com"],
  ]
}

resource "google_compute_url_map" "enx-redirect-urlmap-https" {
  count           = local.enable_enx_redirect ? 1 : 0
  name            = "verification-enx-redirect"
  provider        = google-beta
  project         = var.project
  default_service = google_compute_backend_service.enx-redirect[0].id

  depends_on = [
    google_project_service.services["compute.googleapis.com"],
  ]
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
  ssl_certificates = concat([
    // First certificate is harder to change in UI, so let's keep a separate
    // unused cert in the first slot.
    google_compute_managed_ssl_certificate.enx-redirect-root[0].id,
    google_compute_managed_ssl_certificate.enx-redirect[0].id
    ], var.redirect_cert_generation != var.redirect_cert_generation_next ? [
    google_compute_managed_ssl_certificate.enx-redirect-next[0].id
  ] : [])

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

resource "random_id" "redirect-root-certs" {
  byte_length = 3

  keepers = {
    domains = join(",", local.redirect_root_domains)
  }
}

resource "google_compute_managed_ssl_certificate" "enx-redirect-root" {
  count    = local.enable_enx_redirect ? 1 : 0
  provider = google-beta

  name        = "verification-enx-redirect-root-${random_id.redirect-root-certs.hex}"
  description = "Controlled by Terraform"

  managed {
    domains = local.redirect_root_domains
  }

  # This is to prevent destroying the cert while it's still attached to the load
  # balancer.
  lifecycle {
    create_before_destroy = true
  }

  depends_on = [
    google_project_service.services["compute.googleapis.com"],
  ]
}

resource "google_compute_managed_ssl_certificate" "enx-redirect" {
  count    = local.enable_enx_redirect ? 1 : 0
  provider = google-beta

  name        = format("verification-enx-redirect-cert%s", var.redirect_cert_generation)
  description = "Controlled by Terraform"

  managed {
    // we can only have 100 domains in this list.
    domains = [for o in var.enx_redirect_domain_map : o.host]
  }

  # This is to prevent destroying the cert while it's still attached to the load
  # balancer.
  lifecycle {
    create_before_destroy = true
  }

  depends_on = [
    google_project_service.services["compute.googleapis.com"],
  ]
}

resource "google_compute_managed_ssl_certificate" "enx-redirect-next" {
  count    = var.redirect_cert_generation != var.redirect_cert_generation_next ? 1 : 0
  provider = google-beta

  name        = format("verification-enx-redirect-cert%s", var.redirect_cert_generation_next)
  description = "Controlled by Terraform"

  managed {
    // we can only have 100 domains in this list.
    domains = [for o in concat(var.enx_redirect_domain_map, var.enx_redirect_domain_map_add) : o.host]
  }

  # This is to prevent destroying the cert while it's still attached to the load
  # balancer.
  lifecycle {
    create_before_destroy = true
  }

  depends_on = [
    google_project_service.services["compute.googleapis.com"],
  ]
}
