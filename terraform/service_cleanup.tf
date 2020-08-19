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

resource "google_service_account" "cleanup" {
  project      = var.project
  account_id   = "en-verification-cleanup-sa"
  display_name = "Verification cleanup"
}

resource "google_service_account_iam_member" "cloudbuild-deploy-cleanup" {
  service_account_id = google_service_account.cleanup.id
  role               = "roles/iam.serviceAccountUser"
  member             = "serviceAccount:${data.google_project.project.number}@cloudbuild.gserviceaccount.com"

  depends_on = [
    google_project_service.services["cloudbuild.googleapis.com"],
    google_project_service.services["iam.googleapis.com"],
  ]
}

resource "google_secret_manager_secret_iam_member" "cleanup-db" {
  for_each = toset([
    "sslcert",
    "sslkey",
    "sslrootcert",
    "password",
  ])

  secret_id = google_secret_manager_secret.db-secret[each.key].id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.cleanup.email}"
}

resource "google_project_iam_member" "cleanup-observability" {
  for_each = toset([
    "roles/cloudtrace.agent",
    "roles/logging.logWriter",
    "roles/monitoring.metricWriter",
    "roles/stackdriver.resourceMetadata.writer",
  ])

  project = var.project
  role    = each.key
  member  = "serviceAccount:${google_service_account.cleanup.email}"
}

resource "google_kms_crypto_key_iam_member" "cleanup-database-encrypter" {
  crypto_key_id = google_kms_crypto_key.database-encrypter.self_link
  role          = "roles/cloudkms.cryptoKeyEncrypterDecrypter"
  member        = "serviceAccount:${google_service_account.cleanup.email}"
}

resource "google_secret_manager_secret_iam_member" "cleanup-db-apikey-db-hmac" {
  secret_id = google_secret_manager_secret.db-apikey-db-hmac.id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.cleanup.email}"
}

resource "google_secret_manager_secret_iam_member" "cleanup-db-apikey-sig-hmac" {
  secret_id = google_secret_manager_secret.db-apikey-sig-hmac.id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.cleanup.email}"
}

resource "google_secret_manager_secret_iam_member" "cleanup-db-verification-code-hmac" {
  secret_id = google_secret_manager_secret.db-verification-code-hmac.id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.cleanup.email}"
}

resource "google_cloud_run_service" "cleanup" {
  name     = "cleanup"
  location = var.region

  autogenerate_revision_name = true

  template {
    spec {
      service_account_name = google_service_account.cleanup.email

      containers {
        image = "gcr.io/${var.project}/github.com/google/exposure-notifications-verification-server/cmd/cleanup:initial"

        resources {
          limits = {
            cpu    = "1"
            memory = "512Mi"
          }
        }


        dynamic "env" {
          for_each = merge(
            local.csrf_config,
            local.database_config,
            local.firebase_config,
            local.gcp_config,
            local.signing_config,

            // This MUST come last to allow overrides!
            lookup(var.service_environment, "cleanup", {}),
          )

          content {
            name  = env.key
            value = env.value
          }
        }
      }
    }

    metadata {
      annotations = {
        "run.googleapis.com/vpc-access-connector" : google_vpc_access_connector.connector.id
      }
    }
  }

  depends_on = [
    google_project_service.services["run.googleapis.com"],
    google_secret_manager_secret_iam_member.cleanup-db,
    null_resource.build,
  ]

  lifecycle {
    ignore_changes = [
      template[0].metadata[0].annotations,
      template[0].spec[0].containers[0].image,
    ]
  }
}

output "cleanup_url" {
  value = google_cloud_run_service.cleanup.status.0.url
}

#
# Create scheduler job to invoke the service on a fixed interval.
#

resource "google_service_account" "cleanup-invoker" {
  project      = data.google_project.project.project_id
  account_id   = "en-cleanup-invoker-sa"
  display_name = "Verification cleanup invoker"
}

resource "google_cloud_run_service_iam_member" "cleanup-invoker" {
  project  = google_cloud_run_service.cleanup.project
  location = google_cloud_run_service.cleanup.location
  service  = google_cloud_run_service.cleanup.name
  role     = "roles/run.invoker"
  member   = "serviceAccount:${google_service_account.cleanup-invoker.email}"
}

resource "google_cloud_scheduler_job" "cleanup-worker" {
  name             = "cleanup-worker"
  region           = var.cloudscheduler_location
  schedule         = "0 * * * *"
  time_zone        = "America/Los_Angeles"
  attempt_deadline = "600s"

  retry_config {
    retry_count = 1
  }

  http_target {
    http_method = "GET"
    uri         = "${google_cloud_run_service.cleanup.status.0.url}/"
    oidc_token {
      audience              = google_cloud_run_service.cleanup.status.0.url
      service_account_email = google_service_account.cleanup-invoker.email
    }
  }

  depends_on = [
    google_app_engine_application.app,
    google_cloud_run_service_iam_member.cleanup-invoker,
    google_project_service.services["cloudscheduler.googleapis.com"],
  ]
}
