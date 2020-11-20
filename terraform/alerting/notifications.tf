resource "google_monitoring_notification_channel" "email" {
  provider     = google-beta
  project      = var.monitoring-host-project
  display_name = "Email Notification Channel"
  type         = "email"
  labels       = var.alert-notification-channels
  depends_on = [
    null_resource.manual-step-to-enable-workspace
  ]
}
