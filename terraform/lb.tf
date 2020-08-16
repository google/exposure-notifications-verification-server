# https://github.com/hashicorp/terraform-provider-google-beta/pull/2348
# https://cloud.google.com/load-balancing/docs/negs/setting-up-serverless-negs
resource "google_compute_region_network_endpoint_group" "apiserver-us-central1" {
  provider = google-beta
}

resource "google_compute_http_health_check" "healthz" {
  name               = "healthz"
  request_path       = "/healthz"
  check_interval_sec = 10
  timeout_sec        = 1
}

resource "google_compute_backend_service" "apiserver" {
  name          = "apiserver"
  health_checks = [google_compute_http_health_check.healthz.id]

  backend {
    group = google_compute_region_network_endpoint_group.apiserver-us-central1.id
  }
}

resource "google_compute_global_forwarding_rule" "apiserver-http" {
  name       = "apiserver-http"
  target     = google_compute_backend_service.apiserver.self_link
  ip_address = google_compute_global_address.apiserver.address
  port_range = "80"
  depends_on = [google_compute_global_address.apiserver]
}

resource "google_compute_global_address" "apiserver" {
  name = "apiserver-address"
}
