# Copyright 2022 the Exposure Notifications Verification Server authors
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

resource "google_service_account" "emailer" {
  project      = var.project
  account_id   = "en-verification-emailer-sa"
  display_name = "Verification emailer"
}

resource "google_service_account_iam_member" "cloudbuild-deploy-emailer" {
  service_account_id = google_service_account.emailer.id
  role               = "roles/iam.serviceAccountUser"
  member             = "serviceAccount:${local.cloudbuild_email}"

  depends_on = [
    google_project_service.services["cloudbuild.googleapis.com"],
  ]
}

resource "google_project_iam_member" "emailer-observability" {
  for_each = local.observability_iam_roles
  project  = var.project
  role     = each.key
  member   = "serviceAccount:${google_service_account.emailer.email}"
}

locals {
  emailer_secrets = flatten([
    local.database_secrets,
    local.redis_secrets,
  ])
}

resource "google_secret_manager_secret_iam_member" "emailer-secrets-accessor" {
  count     = length(local.emailer_secrets)
  secret_id = element(local.emailer_secrets, count.index)
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.emailer.email}"
}

resource "google_cloud_run_service" "emailer" {
  name     = "emailer"
  location = var.region

  autogenerate_revision_name = true

  metadata {
    annotations = merge(
      local.default_service_annotations,
      var.default_service_annotations_overrides,
      lookup(var.service_annotations, "emailer", {})
    )
  }

  template {
    spec {
      service_account_name = google_service_account.emailer.email
      timeout_seconds      = 900

      containers {
        image = "gcr.io/${var.project}/github.com/google/exposure-notifications-verification-server/emailer:initial"

        resources {
          limits = {
            cpu    = "1"
            memory = "512Mi"
          }
        }


        dynamic "env" {
          for_each = merge(
            local.database_config,
            local.gcp_config,
            local.emailer_config,
            local.feature_config,
            local.observability_config,

            // This MUST come last to allow overrides!
            lookup(var.service_environment, "_all", {}),
            lookup(var.service_environment, "emailer", {}),
          )

          content {
            name  = env.key
            value = env.value
          }
        }
      }
    }

    metadata {
      annotations = merge(
        local.default_revision_annotations,
        var.default_revision_annotations_overrides,
        // Force all traffic to go through the specific egress IPs, which is
        // required for email authentication.
        {
          "run.googleapis.com/vpc-access-connector" : google_vpc_access_connector.connector.self_link,
          "run.googleapis.com/vpc-access-egress" : "all-traffic",
        },
        lookup(var.revision_annotations, "emailer", {})
      )
    }
  }

  depends_on = [
    google_project_service.services["run.googleapis.com"],

    google_project_iam_member.emailer-observability,
    google_secret_manager_secret_iam_member.emailer-secrets-accessor,

    null_resource.build,
    null_resource.migrate,
  ]

  lifecycle {
    ignore_changes = [
      metadata[0].annotations["client.knative.dev/user-image"],
      metadata[0].annotations["run.googleapis.com/client-name"],
      metadata[0].annotations["run.googleapis.com/client-version"],
      metadata[0].annotations["run.googleapis.com/ingress-status"],
      metadata[0].annotations["serving.knative.dev/creator"],
      metadata[0].annotations["serving.knative.dev/lastModifier"],
      metadata[0].labels["cloud.googleapis.com/location"],
      template[0].metadata[0].annotations["client.knative.dev/user-image"],
      template[0].metadata[0].annotations["run.googleapis.com/client-name"],
      template[0].metadata[0].annotations["run.googleapis.com/client-version"],
      template[0].metadata[0].annotations["serving.knative.dev/creator"],
      template[0].metadata[0].annotations["serving.knative.dev/lastModifier"],
      template[0].spec[0].containers[0].image,
    ]
  }
}

resource "google_service_account" "emailer-invoker" {
  project      = data.google_project.project.project_id
  account_id   = "en-emailer-invoker-sa"
  display_name = "Verification emailer invoker"
}

resource "google_cloud_run_service_iam_member" "emailer-invoker" {
  project  = google_cloud_run_service.emailer.project
  location = google_cloud_run_service.emailer.location
  service  = google_cloud_run_service.emailer.name
  role     = "roles/run.invoker"
  member   = "serviceAccount:${google_service_account.emailer-invoker.email}"
}

resource "google_cloud_scheduler_job" "emailer-anomalies" {
  count = var.enable_emailer ? 1 : 0

  name             = "emailer-anomalies"
  region           = var.cloudscheduler_location
  schedule         = "0 18 * * *"
  time_zone        = var.cloud_scheduler_timezone
  attempt_deadline = "${google_cloud_run_service.emailer.template[0].spec[0].timeout_seconds + 60}s"

  retry_config {
    retry_count = 1
  }

  http_target {
    http_method = "GET"
    uri         = "${google_cloud_run_service.emailer.status.0.url}/anomalies"
    oidc_token {
      audience              = google_cloud_run_service.emailer.status.0.url
      service_account_email = google_service_account.emailer-invoker.email
    }
  }

  depends_on = [
    google_app_engine_application.app,
    google_cloud_run_service_iam_member.emailer-invoker,
    google_project_service.services["cloudscheduler.googleapis.com"],
  ]
}

resource "google_cloud_scheduler_job" "emailer-sms-errors" {
  count = var.enable_emailer ? 1 : 0

  name   = "emailer-sms-errors"
  region = var.cloudscheduler_location

  schedule         = "5 */12 * * *"
  time_zone        = var.cloud_scheduler_timezone
  attempt_deadline = "${google_cloud_run_service.emailer.template[0].spec[0].timeout_seconds + 60}s"

  retry_config {
    retry_count = 1
  }

  http_target {
    http_method = "GET"
    uri         = "${google_cloud_run_service.emailer.status.0.url}/sms-errors"
    oidc_token {
      audience              = google_cloud_run_service.emailer.status.0.url
      service_account_email = google_service_account.emailer-invoker.email
    }
  }

  depends_on = [
    google_app_engine_application.app,
    google_cloud_run_service_iam_member.emailer-invoker,
    google_project_service.services["cloudscheduler.googleapis.com"],
  ]
}
