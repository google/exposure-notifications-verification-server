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
  playbook_prefix = "https://github.com/google/exposure-notifications-verification-server/blob/main/docs/playbooks/alerts"
}

resource "google_monitoring_alert_policy" "RealmTokenRemainingCapacityLow" {
  project      = var.verification-server-project
  combiner     = "OR"
  display_name = "RealmTokenRemainingCapacityLow"
  conditions {
    display_name = "Per-realm issue API token remaining capacity"
    condition_monitoring_query_language {
      duration = "600s"
      query    = <<-EOT
      fetch
      generic_task :: custom.googleapis.com/opencensus/en-verification-server/api/issue/realm_token_latest
      | {
        AVAILABLE: filter metric.state == 'AVAILABLE' | align
        ;
        LIMIT: filter metric.state == 'LIMIT' | align
      }
      | group_by [metric.realm], [val: sum(value.realm_token_latest)]
      | ratio
      | window 1m
      | condition ratio < 0.1
      EOT
      trigger {
        count = 1
      }
    }
  }
  documentation {
    content   = "${local.playbook_prefix}/RealmTokenRemainingCapacityLow.md"
    mime_type = "text/markdown"
  }
  notification_channels = [
    google_monitoring_notification_channel.email.id,
  ]

  depends_on = [
    null_resource.manual-step-to-enable-workspace,
  ]
}

resource "google_monitoring_alert_policy" "backend_latency" {
  count        = var.https-forwarding-rule == "" ? 0 : 1
  project      = var.verification-server-project
  display_name = "ElevatedLatencyGreaterThan2s"
  combiner     = "OR"
  conditions {
    display_name = "/backend_latencies"
    condition_monitoring_query_language {
      duration = "300s"
      query    = <<-EOT
      fetch
      https_lb_rule :: loadbalancing.googleapis.com/https/backend_latencies
      | filter
      (resource.backend_name != 'NO_BACKEND_SELECTED'
      && resource.forwarding_rule_name == '${var.https-forwarding-rule}')
      | align delta(1m)
      | every 1m
      | group_by [resource.backend_target_name], [val: percentile(value.backend_latencies, 99)]
      | condition val > 2000 '1'
      EOT
      trigger {
        count = 1
      }
    }
  }

  documentation {
    content   = "${local.playbook_prefix}/ElevatedLatencyGreaterThan2s.md"
    mime_type = "text/markdown"
  }

  notification_channels = [
    google_monitoring_notification_channel.email.id,
  ]

  depends_on = [
    null_resource.manual-step-to-enable-workspace,
  ]
}

resource "google_monitoring_alert_policy" "E2ETestErrorRatioHigh" {
  project      = var.verification-server-project
  combiner     = "OR"
  display_name = "E2ETestErrorRatioHigh"
  conditions {
    display_name = "E2E test per-step per-test_type error ratio"
    condition_monitoring_query_language {
      duration = "600s"
      query    = <<-EOT
      fetch
      generic_task :: custom.googleapis.com/opencensus/en-verification-server/e2e/request_count
      | {
        NOT_OK: filter metric.result == 'NOT_OK' | align
        ;
        ALL: ident | align
      }
      | group_by [metric.step, metric.test_type], [val: sum(value.request_count)]
      | ratio
      | window 1m
      | condition ratio > 0.1
      EOT
      trigger {
        count = 1
      }
    }
  }
  documentation {
    content   = "${local.playbook_prefix}/E2ETestErrorRatioHigh.md"
    mime_type = "text/markdown"
  }
  notification_channels = [
    google_monitoring_notification_channel.email.id,
  ]

  depends_on = [
    null_resource.manual-step-to-enable-workspace,
  ]
}

resource "google_monitoring_alert_policy" "five_xx" {
  project      = var.monitoring-host-project
  display_name = "Elevated500s"
  combiner     = "OR"
  conditions {
    display_name = "Elevated 5xx on Verification Server"
    condition_monitoring_query_language {
      duration = "300s"
      query    = <<-EOT
      fetch
      cloud_run_revision :: run.googleapis.com/request_count
      | filter
      (resource.service_name != 'e2e-runner')
      && (metric.response_code_class == '5xx')
      | align rate(1m)
      | every 1m
      | group_by [resource.service_name], [val: sum(value.request_count)]
      | condition val > 2 '1/s'
      EOT
      trigger {
        count = 1
      }
    }
  }

  documentation {
    content   = "${local.playbook_prefix}/Elevated500s.md"
    mime_type = "text/markdown"
  }

  notification_channels = [
    google_monitoring_notification_channel.email.id,
  ]

  depends_on = [
    null_resource.manual-step-to-enable-workspace,
  ]
}

resource "google_monitoring_alert_policy" "probers" {
  project = var.monitoring-host-project

  display_name = "HostDown"
  combiner     = "OR"
  conditions {
    display_name = "Host is unreachable"
    condition_monitoring_query_language {
      duration = "300s"
      query    = <<-EOT
      fetch
      uptime_url :: monitoring.googleapis.com/uptime_check/check_passed
      | align next_older(1m)
      | every 1m
      | group_by [resource.host], [val: fraction_true(value.check_passed)]
      | condition val < 0.2 '10^2.%'
      EOT
      trigger {
        count = 1
      }
    }
  }

  documentation {
    content   = "${local.playbook_prefix}/HostDown.md"
    mime_type = "text/markdown"
  }

  notification_channels = [
    google_monitoring_notification_channel.email.id,
  ]

  depends_on = [
    null_resource.manual-step-to-enable-workspace,
  ]
}

resource "google_monitoring_alert_policy" "rate_limited_count" {
  project      = var.monitoring-host-project
  display_name = "ElevatedRateLimitedCount"
  combiner     = "OR"
  conditions {
    display_name = "Rate Limited count by job"
    condition_monitoring_query_language {
      duration = "300s"
      query    = <<-EOT
      fetch
      generic_task :: custom.googleapis.com/opencensus/en-verification-server/ratelimit/limitware/request_count
      | filter metric.result = "RATE_LIMITED"
      | align
      | window 1m
      | group_by [resource.job], [val: sum(value.request_count)]
      | condition val > 1
      EOT
      trigger {
        count = 1
      }
    }
  }

  documentation {
    content   = "${local.playbook_prefix}/ElevatedRateLimitedCount.md"
    mime_type = "text/markdown"
  }

  notification_channels = [
    google_monitoring_notification_channel.email.id,
  ]

  depends_on = [
    null_resource.manual-step-to-enable-workspace,
  ]
}

resource "google_monitoring_alert_policy" "StackdriverExportFailed" {
  project      = var.monitoring-host-project
  display_name = "StackdriverExportFailed"
  combiner     = "OR"
  conditions {
    display_name = "Stackdriver metric export error rate"
    condition_monitoring_query_language {
      duration = "300s"
      query    = <<-EOT
      fetch
      cloud_run_revision::logging.googleapis.com/user/stackdriver_export_error_count
      | align rate(1m)
      | group_by [resource.service_name], [val: sum(value.stackdriver_export_error_count)]
      | condition val > 0.1 '1/s'
      EOT
      trigger {
        count = 1
      }
    }
  }

  documentation {
    content   = "${local.playbook_prefix}/StackdriverExportFailed.md"
    mime_type = "text/markdown"
  }

  notification_channels = [
    google_monitoring_notification_channel.email.id,
  ]

  depends_on = [
    null_resource.manual-step-to-enable-workspace,
    google_logging_metric.stackdriver_export_error_count
  ]
}

# fast error budget burn alert
resource "google_monitoring_alert_policy" "fast_burn" {
  project      = var.verification-server-project
  display_name = "FastErrorBudgetBurn"
  combiner     = "OR"
  enabled      = "true"
  # create only if using GCLB, which means there's an SLO created
  count = var.https-forwarding-rule == "" ? 0 : 1
  conditions {
    display_name = "2% burn in 1 hour"
    condition_threshold {
      filter     = <<-EOT
      select_slo_burn_rate("projects/${var.verification-server-project}/services/verification-server/serviceLevelObjectives/availability-slo", "3600s")
      EOT
      duration   = "0s"
      comparison = "COMPARISON_GT"
      # burn rate = budget consumed * period / alerting window = .02 * (7 * 24 * 60)/60 = 3.36
      threshold_value = 3.36
      trigger {
        count = 1
      }
    }
  }

  documentation {
    content   = "${local.playbook_prefix}/FastErrorBudgetBurn.md"
    mime_type = "text/markdown"
  }

  notification_channels = [
    google_monitoring_notification_channel.email.id,
  ]

  depends_on = [
    google_monitoring_slo.availability-slo
  ]
}
