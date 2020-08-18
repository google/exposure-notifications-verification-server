# https://github.com/hashicorp/terraform-provider-google-beta/pull/2348
# https://cloud.google.com/load-balancing/docs/negs/setting-up-serverless-negs
resource "google_compute_region_network_endpoint_group" "apiserver" {
  name     = "api-server"
  provider = google-beta
  project  = var.project
  region   = var.region

  cloud_run {
    service = google_cloud_run_service.apiserver.name
  }
}

resource "google_compute_backend_service" "apiserver" {
  provider   = google-beta
  name       = "apiserver"
  project    = var.project
  enable_cdn = true

  backend {
    balancing_mode = RATE
    group          = google_compute_region_network_endpoint_group.apiserver.id
  }
}

resource "google_compute_global_address" "verification-server" {
  name    = "verification-server-address"
  project = var.project
}

resource "google_compute_url_map" "urlmap" {
  name            = "verification-server"
  project         = var.project
  default_service = google_compute_backend_service.apiserver.id

  // TODO(icco): Add host base routing for all four services.
}

resource "google_compute_target_http_proxy" "default" {
  name    = "verification-server"
  project = var.project
  url_map = google_compute_url_map.urlmap.id
}

resource "google_compute_forwarding_rule" "verification-server" {
  provider = google-beta
  name     = "verification-server"
  project  = var.project

  ip_protocol           = "TCP"
  ip_address            = google_compute_global_address.verification-server.address
  load_balancing_scheme = "EXTERNAL"
  port_range            = "80"
  target                = google_compute_target_http_proxy.default.id
  network_tier          = "PREMIUM"
}
