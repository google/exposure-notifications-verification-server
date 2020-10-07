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
  all_hosts = toset(concat(var.server_hosts, var.apiserver_hosts, var.adminapi_hosts))
  enable_lb = length(local.all_hosts) > 0
}

resource "google_compute_global_address" "verification-server" {
  count = local.enable_lb ? 1 : 0

  name    = "verification-server-address"
  project = var.project
}

# Redirects all requests to https
resource "google_compute_url_map" "urlmap-http" {
  count = local.enable_lb ? 1 : 0

  name     = "https-redirect"
  provider = google-beta
  project  = var.project

  default_url_redirect {
    strip_query    = false
    https_redirect = true
  }
}

resource "google_compute_url_map" "urlmap-https" {
  count = local.enable_lb ? 1 : 0

  name            = "verification-server"
  provider        = google-beta
  project         = var.project
  default_service = google_compute_backend_service.server[0].id

  // server
  dynamic "host_rule" {
    for_each = length(var.server_hosts) > 0 ? [1] : []

    content {
      path_matcher = "server"
      hosts        = var.server_hosts
    }
  }

  dynamic "path_matcher" {
    for_each = length(var.server_hosts) > 0 ? [1] : []

    content {
      name            = "server"
      default_service = google_compute_backend_service.server[0].id
    }
  }

  // apiserver
  dynamic "host_rule" {
    for_each = length(var.apiserver_hosts) > 0 ? [1] : []

    content {
      path_matcher = "apiserver"
      hosts        = var.apiserver_hosts
    }
  }

  dynamic "path_matcher" {
    for_each = length(var.apiserver_hosts) > 0 ? [1] : []

    content {
      name            = "apiserver"
      default_service = google_compute_backend_service.apiserver[0].id
    }
  }

  // adminapi
  dynamic "host_rule" {
    for_each = length(var.adminapi_hosts) > 0 ? [1] : []

    content {
      path_matcher = "adminapi"
      hosts        = var.adminapi_hosts
    }
  }

  dynamic "path_matcher" {
    for_each = length(var.adminapi_hosts) > 0 ? [1] : []

    content {
      name            = "adminapi"
      default_service = google_compute_backend_service.adminapi[0].id
    }
  }
}

resource "google_compute_target_http_proxy" "http" {
  count = local.enable_lb ? 1 : 0

  provider = google-beta
  name     = "verification-server"
  project  = var.project

  url_map = google_compute_url_map.urlmap-http[0].id
}

resource "google_compute_target_https_proxy" "https" {
  count = local.enable_lb ? 1 : 0

  name    = "verification-server"
  project = var.project

  url_map          = google_compute_url_map.urlmap-https[0].id
  ssl_certificates = [google_compute_managed_ssl_certificate.default[0].id]
}

resource "google_compute_global_forwarding_rule" "http" {
  count = local.enable_lb ? 1 : 0

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
  count = local.enable_lb ? 1 : 0

  provider = google-beta
  name     = "verification-server-https"
  project  = var.project

  ip_protocol           = "TCP"
  ip_address            = google_compute_global_address.verification-server[0].address
  load_balancing_scheme = "EXTERNAL"
  port_range            = "443"
  target                = google_compute_target_https_proxy.https[0].id
}

resource "random_id" "certs" {
  count = local.enable_lb ? 1 : 0

  byte_length = 4

  keepers = {
    domains = join(",", local.all_hosts)
  }
}

resource "google_compute_managed_ssl_certificate" "default" {
  count = local.enable_lb ? 1 : 0

  provider = google-beta
  name     = "verification-certificates-${random_id.certs[0].hex}"
  project  = var.project

  managed {
    domains = local.all_hosts
  }

  # This is to prevent destroying the cert while it's still attached to the load
  # balancer.
  lifecycle {
    create_before_destroy = true
  }
}

output "lb_ip" {
  value = google_compute_global_address.verification-server.*.address
}
