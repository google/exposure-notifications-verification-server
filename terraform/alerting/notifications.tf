resource "google_monitoring_notification_channel" "channels" {
  provider     = google-beta
  project      = var.project
  display_name = "${each.key} Notification Channel"
  type         = each.key
  labels       = each.value.labels
  depends_on = [
    null_resource.manual-step-to-enable-workspace
  ]

  for_each = var.alert-notification-channels
}
