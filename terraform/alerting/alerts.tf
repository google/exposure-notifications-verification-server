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
  p99_latency_thresholds = {
    adminapi = "5s"
  }
  p99_latency_thresholds_default = "2s"

  p99_latency_condition = join("\n  || ", concat(
    [
      for k, v in local.p99_latency_thresholds :
      "(resource.backend_target_name == '${k}' && val > ${replace(v, "/(\\d+)(.*)/", "$1 '$2'")})"
    ],
    [
      "(val > ${replace(local.p99_latency_thresholds_default, "/(\\d+)(.*)/", "$1 '$2'")})"
    ]
  ))
}

resource "google_monitoring_alert_policy" "E2ETestErrorRatioHigh" {
  project      = var.project
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
      | condition ratio > 10 '%'
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
  notification_channels = [for x in values(google_monitoring_notification_channel.channels) : x.id]

  depends_on = [
    null_resource.manual-step-to-enable-workspace,
  ]
}

resource "google_monitoring_alert_policy" "five_xx" {
  project      = var.project
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

  notification_channels = [for x in values(google_monitoring_notification_channel.channels) : x.id]

  depends_on = [
    null_resource.manual-step-to-enable-workspace,
  ]
}

resource "google_monitoring_alert_policy" "probers" {
  project = var.project

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
      | condition val < 20 '%'
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

  notification_channels = [for x in values(google_monitoring_notification_channel.channels) : x.id]

  depends_on = [
    null_resource.manual-step-to-enable-workspace,
  ]
}

resource "google_monitoring_alert_policy" "rate_limited_count" {
  project      = var.project
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

  notification_channels = [for x in values(google_monitoring_notification_channel.channels) : x.id]

  depends_on = [
    null_resource.manual-step-to-enable-workspace,
  ]
}

resource "google_monitoring_alert_policy" "StackdriverExportFailed" {
  project      = var.project
  display_name = "StackdriverExportFailed"
  combiner     = "OR"
  conditions {
    display_name = "Stackdriver metric export error rate"
    condition_monitoring_query_language {
      duration = "900s"
      # NOTE: this query calculates the rate over a 5min window instead of
      # usual 1min window. This is intentional:
      # The rate window should be larger than the interval of the errors.
      # Currently we export to stackdriver every 2min, meaning if the export is
      # constantly failing, our calculated error rate with 1min window will
      # have the number oscillating between 0 and 1, and we would never get an
      # alert beacuase each time the value reaches 0 the timer to trigger the
      # alert is reset.
      #
      # Changing this to 5min window means the condition is "on" as soon as
      # there's a single export error and last at least 5min. The alert is
      # firing if the condition is "on" for >15min.
      query = <<-EOT
      fetch
      cloud_run_revision::logging.googleapis.com/user/stackdriver_export_error_count
      | align rate(5m)
      | group_by [resource.service_name], [val: sum(value.stackdriver_export_error_count)]
      | condition val > 0
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

  notification_channels = [for x in values(google_monitoring_notification_channel.channels) : x.id]

  depends_on = [
    null_resource.manual-step-to-enable-workspace,
    google_logging_metric.stackdriver_export_error_count
  ]
}

resource "google_monitoring_alert_policy" "CloudSchedulerJobFailed" {
  project      = var.project
  display_name = "CloudSchedulerJobFailed"
  combiner     = "OR"
  conditions {
    display_name = "Cloud Scheduler Job Error Ratio"
    condition_monitoring_query_language {
      duration = "900s"
      # Uses rate(5m). See the reasoning above.
      query = <<-EOT
      fetch cloud_scheduler_job::logging.googleapis.com/log_entry_count
      | filter (metric.severity == 'ERROR')
      | align rate(5m)
      | group_by [resource.job_id], [val: aggregate(value.log_entry_count)]
      | condition val > 0
      EOT
      trigger {
        count = 1
      }
    }
  }

  documentation {
    content   = "${local.playbook_prefix}/CloudSchedulerJobFailed.md"
    mime_type = "text/markdown"
  }

  notification_channels = [for x in values(google_monitoring_notification_channel.channels) : x.id]

  depends_on = [
    null_resource.manual-step-to-enable-workspace,
  ]
}

resource "google_monitoring_alert_policy" "HumanAccessedSecret" {
  count = var.alert_on_human_accessed_secret ? 1 : 0

  project      = var.project
  display_name = "HumanAccessedSecret"
  combiner     = "OR"

  conditions {
    display_name = "A non-service account accessed a secret."

    condition_monitoring_query_language {
      duration = "60s"

      query = <<-EOT
      fetch audited_resource
      | metric 'logging.googleapis.com/user/human_accessed_secret'
      | align rate(5m)
      | every 1m
      | group_by [resource.project_id],
          [value_human_accessed_secret_aggregate: aggregate(value.human_accessed_secret)]
      | condition value_human_accessed_secret_aggregate > 0
      EOT

      trigger {
        count = 1
      }
    }
  }

  documentation {
    content   = "${local.playbook_prefix}/HumanAccessedSecret.md"
    mime_type = "text/markdown"
  }

  notification_channels = [for x in values(google_monitoring_notification_channel.channels) : x.id]

  depends_on = [
    null_resource.manual-step-to-enable-workspace,
  ]
}

resource "google_monitoring_alert_policy" "HumanDecryptedValue" {
  count = var.alert_on_human_decrypted_value ? 1 : 0

  project      = var.project
  display_name = "HumanDecryptedValue"
  combiner     = "OR"

  conditions {
    display_name = "A non-service account decrypted something."

    condition_monitoring_query_language {
      duration = "60s"

      query = <<-EOT
      fetch audited_resource
      | metric 'logging.googleapis.com/user/human_decrypted_value'
      | align rate(5m)
      | every 1m
      | group_by [resource.project_id],
          [value_human_decrypted_value_aggregate: aggregate(value.human_decrypted_value)]
      | condition value_human_decrypted_value_aggregate > 0
      EOT

      trigger {
        count = 1
      }
    }
  }

  documentation {
    content   = "${local.playbook_prefix}/HumanDecryptedValue.md"
    mime_type = "text/markdown"
  }

  notification_channels = [for x in values(google_monitoring_notification_channel.channels) : x.id]

  depends_on = [
    null_resource.manual-step-to-enable-workspace,
  ]
}
