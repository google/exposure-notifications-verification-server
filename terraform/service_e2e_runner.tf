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

resource "google_service_account" "e2e-runner" {
  project      = var.project
  account_id   = "en-verification-e2e-runner-sa"
  display_name = "Verification e2e-runner"
}

resource "google_service_account_iam_member" "cloudbuild-deploy-e2e-runner" {
  service_account_id = google_service_account.e2e-runner.id
  role               = "roles/iam.serviceAccountUser"
  member             = "serviceAccount:${local.cloudbuild_email}"
}

resource "google_project_iam_member" "e2e-runner-observability" {
  for_each = local.observability_iam_roles
  project  = var.project
  role     = each.key
  member   = "serviceAccount:${google_service_account.e2e-runner.email}"
}

resource "google_kms_crypto_key_iam_member" "e2e-runner-database-encrypter" {
  crypto_key_id = google_kms_crypto_key.database-encrypter.self_link
  role          = "roles/cloudkms.cryptoKeyEncrypterDecrypter"
  member        = "serviceAccount:${google_service_account.e2e-runner.email}"
}

locals {
  e2e_runner_secrets = flatten([
    local.database_secrets,
    local.redis_secrets,
  ])
}

resource "google_secret_manager_secret_iam_member" "e2e-runner-secrets" {
  count     = length(local.e2e_runner_secrets)
  secret_id = element(local.e2e_runner_secrets, count.index)
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.e2e-runner.email}"
}

resource "google_cloud_run_service" "e2e-runner" {
  name     = "e2e-runner"
  location = var.region

  autogenerate_revision_name = true

  metadata {
    annotations = merge(
      local.default_service_annotations,
      var.default_service_annotations_overrides,
      lookup(var.service_annotations, "e2e-runner", {})
    )
  }

  template {
    spec {
      service_account_name = google_service_account.e2e-runner.email
      timeout_seconds      = 120

      containers {
        image = "gcr.io/${var.project}/github.com/google/exposure-notifications-verification-server/e2e-runner:initial"

        resources {
          limits = {
            cpu    = "1"
            memory = "512Mi"
          }
        }


        dynamic "env" {
          for_each = merge(
            local.cache_config,
            local.database_config,
            local.firebase_config,
            local.gcp_config,
            local.signing_config,
            local.e2e_runner_config,
            local.observability_config,

            // This MUST come last to allow overrides!
            lookup(var.service_environment, "_all", {}),
            lookup(var.service_environment, "e2e-runner", {}),
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
        lookup(var.revision_annotations, "e2e-runner", {})
      )
    }
  }

  depends_on = [
    google_project_service.services["run.googleapis.com"],

    google_kms_crypto_key_iam_member.e2e-runner-database-encrypter,
    google_project_iam_member.e2e-runner-observability,
    google_secret_manager_secret_iam_member.e2e-runner-secrets,

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

output "e2e_runner_url" {
  value = google_cloud_run_service.e2e-runner.status.0.url
}

#
# Create scheduler job to invoke the service on a fixed interval.
#

resource "google_service_account" "e2e-runner-invoker" {
  project      = data.google_project.project.project_id
  account_id   = "en-e2e-runner-invoker-sa"
  display_name = "Verification e2e-runner invoker"
}

resource "google_cloud_run_service_iam_member" "e2e-runner-invoker" {
  project  = google_cloud_run_service.e2e-runner.project
  location = google_cloud_run_service.e2e-runner.location
  service  = google_cloud_run_service.e2e-runner.name
  role     = "roles/run.invoker"
  member   = "serviceAccount:${google_service_account.e2e-runner-invoker.email}"
}

resource "google_cloud_scheduler_job" "e2e-default-workflow" {
  name             = "e2e-default-workflow"
  region           = var.cloudscheduler_location
  schedule         = "*/5 * * * *"
  time_zone        = "America/Los_Angeles"
  attempt_deadline = "${google_cloud_run_service.e2e-runner.template[0].spec[0].timeout_seconds + 60}s"

  retry_config {
    retry_count = 3
  }

  http_target {
    http_method = "GET"
    uri         = "${google_cloud_run_service.e2e-runner.status.0.url}/default"
    oidc_token {
      audience              = "${google_cloud_run_service.e2e-runner.status.0.url}/default"
      service_account_email = google_service_account.e2e-runner-invoker.email
    }
  }

  depends_on = [
    google_app_engine_application.app,
    google_cloud_run_service_iam_member.e2e-runner-invoker,
    google_project_service.services["cloudscheduler.googleapis.com"],
  ]
}

resource "google_cloud_scheduler_job" "e2e-revise-workflow" {
  name             = "e2e-revise-workflow"
  region           = var.cloudscheduler_location
  schedule         = "2-59/5 * * * *"
  time_zone        = "America/Los_Angeles"
  attempt_deadline = "${google_cloud_run_service.e2e-runner.template[0].spec[0].timeout_seconds + 60}s"

  retry_config {
    retry_count = 3
  }

  http_target {
    http_method = "GET"
    uri         = "${google_cloud_run_service.e2e-runner.status.0.url}/revise"
    oidc_token {
      audience              = "${google_cloud_run_service.e2e-runner.status.0.url}/revise"
      service_account_email = google_service_account.e2e-runner-invoker.email
    }
  }

  depends_on = [
    google_app_engine_application.app,
    google_cloud_run_service_iam_member.e2e-runner-invoker,
    google_project_service.services["cloudscheduler.googleapis.com"],
  ]
}

resource "google_cloud_scheduler_job" "e2e-user-report-workflow" {
  name             = "e2e-user-report-workflow"
  region           = var.cloudscheduler_location
  schedule         = "3-59/5 * * * *"
  time_zone        = "America/Los_Angeles"
  attempt_deadline = "${google_cloud_run_service.e2e-runner.template[0].spec[0].timeout_seconds + 60}s"

  retry_config {
    retry_count = 3
  }

  http_target {
    http_method = "GET"
    uri         = "${google_cloud_run_service.e2e-runner.status.0.url}/user-report"
    oidc_token {
      audience              = "${google_cloud_run_service.e2e-runner.status.0.url}/user-report"
      service_account_email = google_service_account.e2e-runner-invoker.email
    }
  }

  depends_on = [
    google_app_engine_application.app,
    google_cloud_run_service_iam_member.e2e-runner-invoker,
    google_project_service.services["cloudscheduler.googleapis.com"],
  ]
}

resource "google_cloud_scheduler_job" "e2e-enx-redirect-workflow" {
  name             = "e2e-enx-redirect-workflow"
  region           = var.cloudscheduler_location
  schedule         = "*/5 * * * *"
  time_zone        = "America/Los_Angeles"
  attempt_deadline = "${google_cloud_run_service.e2e-runner.template[0].spec[0].timeout_seconds + 60}s"

  retry_config {
    retry_count = 3
  }

  http_target {
    http_method = "GET"
    uri         = "${google_cloud_run_service.e2e-runner.status.0.url}/enx-redirect"
    oidc_token {
      audience              = "${google_cloud_run_service.e2e-runner.status.0.url}/enx-redirect"
      service_account_email = google_service_account.e2e-runner-invoker.email
    }
  }

  depends_on = [
    google_app_engine_application.app,
    google_cloud_run_service_iam_member.e2e-runner-invoker,
    google_project_service.services["cloudscheduler.googleapis.com"],
  ]
}
