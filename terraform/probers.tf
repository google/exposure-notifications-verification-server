resource "google_monitoring_uptime_check_config" "https" {
  for_each = toset([var.server-host, var.apiserver-host, var.adminapi-host])

  display_name = each.key
  timeout      = "3s"
  project      = var.project
  period       = "60s"

  http_check {
    path         = "/health"
    port         = "443"
    use_ssl      = true
    validate_ssl = true
  }

  monitored_resource {
    type = "uptime_url"
    labels = {
      project_id = var.project
      host       = each.key
    }
  }
}

resource "google_monitoring_alert_policy" "probers" {
  project      = var.project
  display_name = "Host Down"
  combiner     = "OR"
  conditions {
    display_name = "Host is unreachable"
    condition_threshold {
      duration        = "300s"
      threshold_value = 0.2
      comparison      = "COMPARISON_LT"
      filter          = "metric.type=\"monitoring.googleapis.com/uptime_check/check_passed\" resource.type=\"uptime_url\""

      aggregations {
        alignment_period     = "60s"
        cross_series_reducer = "REDUCE_FRACTION_TRUE"
        group_by_fields = [
          "resource.label.host",
        ]
        per_series_aligner = "ALIGN_NEXT_OLDER"
      }

      trigger {
        count = 1
      }
    }
  }

  documentation {
    content   = <<-EOT
## $${policy.display_name}

[$${resource.label.host}](https://$${resource.label.host}/health) is being reported unreachable.

See [docs/hosts.md](https://github.com/sethvargo/exposure-notifications-server-infra/blob/main/docs/hosts.md) for information about hosts.
EOT
    mime_type = "text/markdown"
  }

  notification_channels = [
    google_monitoring_notification_channel.email.id
  ]
}
