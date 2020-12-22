resource "google_monitoring_custom_service" "service" {
  service_id   = var.service_name
  display_name = var.service_name
  project      = var.project
}
