# https://github.com/hashicorp/terraform-provider-google-beta/pull/2348
# https://cloud.google.com/load-balancing/docs/negs/setting-up-serverless-negs
resource "google_compute_region_network_endpoint_group" "apiserver-us-central1" {
  provider  = google-beta
  region    = "us-central1"
  cloud_run = "apiserver"
}

resource "google_compute_backend_service" "apiserver" {
  name       = "apiserver"
  enable_cdn = true

  backend {
    group = google_compute_region_network_endpoint_group.apiserver-us-central1.id
  }
}

resource "google_compute_global_address" "verification-server" {
  name = "verification-server-address"
}

resource "google_compute_url_map" "urlmap" {
  name = "verification-server"

  default_service = google_compute_backend_service.apiserver.id

  // TODO(icco): Add host base routing for all four services.
}

resource "google_compute_target_http_proxy" "default" {
  url_map = google_compute_url_map.urlmap.id
}

resource "google_compute_forwarding_rule" "verification-server" {
  provider = google-beta
  name     = "verification-server"

  ip_protocol           = "TCP"
  ip_address            = google_compute_global_address.verification-server.address
  load_balancing_scheme = "EXTERNAL"
  port_range            = "80"
  target                = google_compute_region_target_http_proxy.default.id
  network_tier          = "PREMIUM"
}
