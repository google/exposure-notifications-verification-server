# Copyright 2021 Google LLC
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

resource "google_service_account" "rotation" {
  project      = var.project
  account_id   = "en-verification-rotation-sa"
  display_name = "Verification rotation"
}

resource "google_service_account_iam_member" "cloudbuild-deploy-rotation" {
  service_account_id = google_service_account.rotation.id
  role               = "roles/iam.serviceAccountUser"
  member             = "serviceAccount:${data.google_project.project.number}@cloudbuild.gserviceaccount.com"

  depends_on = [
    google_project_service.services["cloudbuild.googleapis.com"],
    google_project_service.services["iam.googleapis.com"],
  ]
}

resource "google_secret_manager_secret_iam_member" "rotation-db" {
  for_each = toset([
    "sslcert",
    "sslkey",
    "sslrootcert",
    "password",
  ])

  secret_id = google_secret_manager_secret.db-secret[each.key].id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.rotation.email}"
}

resource "google_project_iam_member" "rotation-observability" {
  for_each = toset([
    "roles/cloudtrace.agent",
    "roles/logging.logWriter",
    "roles/monitoring.metricWriter",
    "roles/stackdriver.resourceMetadata.writer",
  ])

  project = var.project
  role    = each.key
  member  = "serviceAccount:${google_service_account.rotation.email}"
}

resource "google_kms_crypto_key_iam_member" "rotation-database-encrypter" {
  crypto_key_id = google_kms_crypto_key.database-encrypter.self_link
  role          = "roles/cloudkms.cryptoKeyEncrypterDecrypter"
  member        = "serviceAccount:${google_service_account.rotation.email}"
}

resource "google_secret_manager_secret_iam_member" "rotation-db-apikey-db-hmac" {
  secret_id = google_secret_manager_secret.db-apikey-db-hmac.id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.rotation.email}"
}

resource "google_secret_manager_secret_iam_member" "rotation-db-apikey-sig-hmac" {
  secret_id = google_secret_manager_secret.db-apikey-sig-hmac.id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.rotation.email}"
}

resource "google_secret_manager_secret_iam_member" "rotation-db-verification-code-hmac" {
  secret_id = google_secret_manager_secret.db-verification-code-hmac.id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.rotation.email}"
}

resource "google_secret_manager_secret_iam_member" "rotation-cache-hmac-key" {
  secret_id = google_secret_manager_secret.cache-hmac-key.id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.rotation.email}"
}

resource "google_secret_manager_secret_iam_member" "rotation-ratelimit-hmac-key" {
  secret_id = google_secret_manager_secret.ratelimit-hmac-key.id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.rotation.email}"
}

resource "google_cloud_run_service" "rotation" {
  name     = "rotation"
  location = var.region

  autogenerate_revision_name = true

  metadata {
    annotations = merge(
      local.default_service_annotations,
      var.default_service_annotations_overrides,
      lookup(var.service_annotations, "rotation", {})
    )
  }
  template {
    spec {
      service_account_name = google_service_account.rotation.email

      containers {
        image = "gcr.io/${var.project}/github.com/google/exposure-notifications-verification-server/rotation:initial"

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
            local.signing_config,
            local.observability_config,

            // This MUST come last to allow overrides!
            lookup(var.service_environment, "rotation", {}),
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
        lookup(var.revision_annotations, "rotation", {})
      )
    }
  }

  depends_on = [
    google_project_service.services["run.googleapis.com"],

    google_secret_manager_secret_iam_member.rotation-db,
    google_project_iam_member.rotation-observability,
    google_kms_crypto_key_iam_member.rotation-database-encrypter,
    google_secret_manager_secret_iam_member.rotation-db-apikey-db-hmac,
    google_secret_manager_secret_iam_member.rotation-db-apikey-sig-hmac,
    google_secret_manager_secret_iam_member.rotation-db-verification-code-hmac,
    google_secret_manager_secret_iam_member.rotation-cache-hmac-key,
    google_secret_manager_secret_iam_member.rotation-ratelimit-hmac-key,

    null_resource.build,
    null_resource.migrate,
  ]

  lifecycle {
    ignore_changes = [
      template[0].metadata[0].annotations["client.knative.dev/user-image"],
      template[0].metadata[0].annotations["run.googleapis.com/client-name"],
      template[0].metadata[0].annotations["run.googleapis.com/client-version"],
      template[0].spec[0].containers[0].image,
      metadata[0].annotations["run.googleapis.com/ingress-status"],
      metadata[0].labels["cloud.googleapis.com/location"],
    ]
  }
}

output "rotation_url" {
  value = google_cloud_run_service.rotation.status.0.url
}

#
# Create scheduler job to invoke the service on a fixed interval.
#

resource "google_service_account" "rotation-invoker" {
  project      = data.google_project.project.project_id
  account_id   = "en-rotation-invoker-sa"
  display_name = "Verification rotation invoker"
}

resource "google_cloud_run_service_iam_member" "rotation-invoker" {
  project  = google_cloud_run_service.rotation.project
  location = google_cloud_run_service.rotation.location
  service  = google_cloud_run_service.rotation.name
  role     = "roles/run.invoker"
  member   = "serviceAccount:${google_service_account.rotation-invoker.email}"
}

resource "google_cloud_scheduler_job" "rotation-worker" {
  name             = "rotation-worker"
  region           = var.cloudscheduler_location
  schedule         = "*/5 * * * *"
  time_zone        = "America/Los_Angeles"
  attempt_deadline = "600s"

  retry_config {
    retry_count = 1
  }

  http_target {
    http_method = "GET"
    uri         = "${google_cloud_run_service.rotation.status.0.url}/"
    oidc_token {
      audience              = google_cloud_run_service.rotation.status.0.url
      service_account_email = google_service_account.rotation-invoker.email
    }
  }

  depends_on = [
    google_app_engine_application.app,
    google_cloud_run_service_iam_member.rotation-invoker,
    google_project_service.services["cloudscheduler.googleapis.com"],
  ]
}
