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

resource "google_service_account" "appsync" {
  project      = var.project
  account_id   = "en-ver-appsync-sa"
  display_name = "Verification App Sync"
}

resource "google_service_account_iam_member" "cloudbuild-deploy-appsync" {
  service_account_id = google_service_account.appsync.id
  role               = "roles/iam.serviceAccountUser"
  member             = "serviceAccount:${local.cloudbuild_email}"
}

resource "google_project_iam_member" "appsync-observability" {
  for_each = local.observability_iam_roles
  project  = var.project
  role     = each.key
  member   = "serviceAccount:${google_service_account.appsync.email}"
}

resource "google_kms_crypto_key_iam_member" "appsync-database-encrypter" {
  crypto_key_id = google_kms_crypto_key.database-encrypter.self_link
  role          = "roles/cloudkms.cryptoKeyEncrypterDecrypter"
  member        = "serviceAccount:${google_service_account.appsync.email}"
}

locals {
  appsync_secrets = flatten([
    local.database_secrets,
    local.redis_secrets,
  ])
}

resource "google_secret_manager_secret_iam_member" "appsync-secrets" {
  count     = length(local.appsync_secrets)
  secret_id = element(local.appsync_secrets, count.index)
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.appsync.email}"
}

resource "google_cloud_run_service" "appsync" {
  name     = "appsync"
  location = var.region

  autogenerate_revision_name = true

  metadata {
    annotations = merge(
      local.default_service_annotations,
      var.default_service_annotations_overrides,
      lookup(var.service_annotations, "appsync", {})
    )
  }
  template {
    spec {
      service_account_name = google_service_account.appsync.email

      containers {
        image = "gcr.io/${var.project}/github.com/google/exposure-notifications-verification-server/appsync:initial"

        resources {
          limits = {
            cpu    = "1"
            memory = "512Mi"
          }
        }


        dynamic "env" {
          for_each = merge(
            local.appsync_config,
            local.cache_config,
            local.csrf_config,
            local.database_config,
            local.firebase_config,
            local.gcp_config,
            local.signing_config,
            local.observability_config,

            // This MUST come last to allow overrides!
            lookup(var.service_environment, "appsync", {}),
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
        lookup(var.revision_annotations, "appsync", {})
      )
    }
  }

  depends_on = [
    google_project_service.services["run.googleapis.com"],

    google_kms_crypto_key_iam_member.appsync-database-encrypter,
    google_project_iam_member.appsync-observability,
    google_secret_manager_secret_iam_member.appsync-secrets,

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

output "appsync_url" {
  value = google_cloud_run_service.appsync.status.0.url
}

#
# Create scheduler job to invoke the service on a fixed interval.
#

resource "google_service_account" "appsync-invoker" {
  project      = data.google_project.project.project_id
  account_id   = "en-appsync-invk-sa"
  display_name = "Verification appsync invoker"
}

resource "google_cloud_run_service_iam_member" "appsync-invoker" {
  project  = google_cloud_run_service.appsync.project
  location = google_cloud_run_service.appsync.location
  service  = google_cloud_run_service.appsync.name
  role     = "roles/run.invoker"
  member   = "serviceAccount:${google_service_account.appsync-invoker.email}"
}

resource "google_cloud_scheduler_job" "appsync-worker" {
  name             = "appsync-worker"
  region           = var.cloudscheduler_location
  schedule         = "20 11 * * *"
  time_zone        = "America/Los_Angeles"
  attempt_deadline = "600s"

  retry_config {
    retry_count = 3
  }

  http_target {
    http_method = "GET"
    uri         = "${google_cloud_run_service.appsync.status.0.url}/"
    oidc_token {
      audience              = google_cloud_run_service.appsync.status.0.url
      service_account_email = google_service_account.appsync-invoker.email
    }
  }

  depends_on = [
    google_app_engine_application.app,
    google_cloud_run_service_iam_member.appsync-invoker,
    google_project_service.services["cloudscheduler.googleapis.com"],
  ]
}
