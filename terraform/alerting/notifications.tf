resource "google_monitoring_notification_channel" "paging" {
  provider     = google-beta
  project      = var.project
  display_name = "Paging Notification Channel"
  type         = each.key
  labels       = each.value.labels
  depends_on = [
    null_resource.manual-step-to-enable-workspace
  ]

  for_each = var.alert-notification-channel-paging
}

resource "google_monitoring_notification_channel" "non-paging" {
  provider     = google-beta
  project      = var.project
  display_name = "Non-paging Notification Channel"
  type         = each.key
  labels       = each.value.labels
  depends_on = [
    null_resource.manual-step-to-enable-workspace
  ]

  for_each = var.alert-notification-channel-non-paging
}
