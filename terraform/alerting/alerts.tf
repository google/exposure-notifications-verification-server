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
  custom_prefix   = "custom.googleapis.com/opencensus/en-verification-server"

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
      generic_task :: ${local.custom_prefix}/e2e/request_count
      | {
        NOT_OK: filter metric.result == 'NOT_OK' | align
        ;
        ALL: ident | align
      }
      | group_by [metric.step, metric.test_type], [val: sum(value.request_count)]
      | ratio
      | window 1m
      | condition ratio > 5 '%'
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
  notification_channels = [for x in values(google_monitoring_notification_channel.paging) : x.id]

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

  notification_channels = [for x in values(google_monitoring_notification_channel.paging) : x.id]

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
      generic_task :: ${local.custom_prefix}/ratelimit/limitware/request_count
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

  notification_channels = [for x in values(google_monitoring_notification_channel.non-paging) : x.id]

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

  notification_channels = [for x in values(google_monitoring_notification_channel.non-paging) : x.id]

  depends_on = [
    null_resource.manual-step-to-enable-workspace,
    google_logging_metric.stackdriver_export_error_count
  ]
}

resource "google_monitoring_alert_policy" "ForwardProgressFailed" {
  for_each = var.forward_progress_indicators

  project      = var.project
  display_name = "ForwardProgressFailed-${each.key}"
  combiner     = "OR"

  conditions {
    display_name = each.key

    condition_monitoring_query_language {
      duration = "0s"
      query    = <<-EOT
      fetch generic_task
      | metric '${local.custom_prefix}/${each.value.metric}'
      | align delta_gauge(${each.value.window})
      | group_by [], [val: aggregate(value.success)]
      | absent_for ${each.value.window}
      EOT

      trigger {
        count = 1
      }
    }
  }

  documentation {
    content   = "${local.playbook_prefix}/ForwardProgressFailed.md"
    mime_type = "text/markdown"
  }

  notification_channels = [for x in values(google_monitoring_notification_channel.paging) : x.id]

  depends_on = [
    null_resource.manual-step-to-enable-workspace,
  ]
}

resource "google_monitoring_alert_policy" "UpstreamUserRecreates" {
  project      = var.project
  combiner     = "OR"
  display_name = "UpstreamUserRecreates"
  conditions {
    display_name = "Upstream users that should have existed but did not and we re-created"
    condition_monitoring_query_language {
      duration = "600s"
      query    = <<-EOT
      fetch
      generic_task :: ${local.custom_prefix}/user/upstream_user_recreate_count
      | align rate(5m)
      | every 1m
      | group_by [], [val: sum(value.upstream_user_recreate_count)]
      | condition val > 5 '1/s'
      EOT
      trigger {
        count = 1
      }
    }
  }
  documentation {
    content   = "${local.playbook_prefix}/UpstreamUserRecreates.md"
    mime_type = "text/markdown"
  }
  notification_channels = [for x in values(google_monitoring_notification_channel.non-paging) : x.id]

  depends_on = [
    null_resource.manual-step-to-enable-workspace,
  ]
}

resource "google_monitoring_alert_policy" "AuthenticatedSMSFailure" {
  project      = var.project
  combiner     = "OR"
  display_name = "AuthenticatedSMSFailure"
  conditions {
    display_name = "Failed attempts to sign an SMS for authenticated SMS"
    condition_monitoring_query_language {
      duration = "60s"
      query    = <<-EOT
      fetch
      generic_task :: ${local.custom_prefix}/api/issue/authenticated_sms_failure_count
      | align rate(5m)
      | every 1m
      | group_by [metric.realm], [val: sum(value.authenticated_sms_failure_count)]
      | condition val > 5 '1/s'
      EOT
      trigger {
        count = 1
      }
    }
  }
  documentation {
    content   = "${local.playbook_prefix}/AuthenticatedSMSFailure.md"
    mime_type = "text/markdown"
  }
  notification_channels = [for x in values(google_monitoring_notification_channel.non-paging) : x.id]

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
      duration = "0s"

      query = <<-EOT
      fetch audited_resource
      | metric 'logging.googleapis.com/user/${google_logging_metric.human_accessed_secret.name}'
      | align rate(5m)
      | every 1m
      | group_by [resource.project_id],
          [val: aggregate(value.human_accessed_secret)]
      | condition val > 0
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

  notification_channels = [for x in values(google_monitoring_notification_channel.paging) : x.id]

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
      duration = "0s"

      query = <<-EOT
      fetch global
      | metric 'logging.googleapis.com/user/${google_logging_metric.human_decrypted_value.name}'
      | align rate(5m)
      | every 1m
      | group_by [resource.project_id],
          [val: aggregate(value.human_decrypted_value)]
      | condition val > 0
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

  notification_channels = [for x in values(google_monitoring_notification_channel.paging) : x.id]

  depends_on = [
    null_resource.manual-step-to-enable-workspace,
  ]
}
