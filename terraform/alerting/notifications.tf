resource "google_monitoring_notification_channel" "email" {
  provider     = google-beta
  project      = var.project
  display_name = "Email Notification Channel"
  type         = "email"
  labels = {
    email_address = var.notification-email
  }
}
