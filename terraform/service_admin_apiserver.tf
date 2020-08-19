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

resource "google_service_account" "adminapi" {
  project      = var.project
  account_id   = "en-verification-adminapi-sa"
  display_name = "Verification Admin apiserver"
}

resource "google_service_account_iam_member" "cloudbuild-deploy-adminapi" {
  service_account_id = google_service_account.adminapi.id
  role               = "roles/iam.serviceAccountUser"
  member             = "serviceAccount:${data.google_project.project.number}@cloudbuild.gserviceaccount.com"

  depends_on = [
    google_project_service.services["cloudbuild.googleapis.com"],
    google_project_service.services["iam.googleapis.com"],
  ]
}

resource "google_secret_manager_secret_iam_member" "adminapi-db" {
  for_each = toset([
    "sslcert",
    "sslkey",
    "sslrootcert",
    "password",
  ])

  secret_id = google_secret_manager_secret.db-secret[each.key].id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.adminapi.email}"
}

resource "google_project_iam_member" "adminapi-observability" {
  for_each = toset([
    "roles/cloudtrace.agent",
    "roles/logging.logWriter",
    "roles/monitoring.metricWriter",
    "roles/stackdriver.resourceMetadata.writer",
  ])

  project = var.project
  role    = each.key
  member  = "serviceAccount:${google_service_account.adminapi.email}"
}

resource "google_kms_crypto_key_iam_member" "adminapi-database-encrypter" {
  crypto_key_id = google_kms_crypto_key.database-encrypter.self_link
  role          = "roles/cloudkms.cryptoKeyEncrypterDecrypter"
  member        = "serviceAccount:${google_service_account.adminapi.email}"
}

resource "google_secret_manager_secret_iam_member" "adminapi-db-apikey-db-hmac" {
  secret_id = google_secret_manager_secret.db-apikey-db-hmac.id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.adminapi.email}"
}

resource "google_secret_manager_secret_iam_member" "adminapi-db-apikey-sig-hmac" {
  secret_id = google_secret_manager_secret.db-apikey-sig-hmac.id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.adminapi.email}"
}

resource "google_secret_manager_secret_iam_member" "adminapi-db-verification-code-hmac" {
  secret_id = google_secret_manager_secret.db-verification-code-hmac.id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.adminapi.email}"
}

resource "google_cloud_run_service" "adminapi" {
  name     = "adminapi"
  location = var.region

  autogenerate_revision_name = true

  template {
    spec {
      service_account_name = google_service_account.adminapi.email

      containers {
        image = "gcr.io/${var.project}/github.com/google/exposure-notifications-verification-server/cmd/adminapi:initial"

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
            local.redis_config,

            // This MUST come last to allow overrides!
            lookup(var.service_environment, "adminapi", {}),
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
    google_secret_manager_secret_iam_member.adminapi-db,
    null_resource.build,
  ]

  lifecycle {
    ignore_changes = [
      template[0].metadata[0].annotations,
      template[0].spec[0].containers[0].image,
    ]
  }
}

resource "google_cloud_run_domain_mapping" "adminapi" {
  count    = var.adminapi_custom_domain != "" ? 1 : 0
  location = var.cloudrun_location
  name     = var.adminapi_custom_domain

  metadata {
    namespace = var.project
  }

  spec {
    route_name     = google_cloud_run_service.adminapi.name
    force_override = true
  }
}

resource "google_cloud_run_service_iam_member" "adminapi-public" {
  location = google_cloud_run_service.adminapi.location
  project  = google_cloud_run_service.adminapi.project
  service  = google_cloud_run_service.adminapi.name
  role     = "roles/run.invoker"
  member   = "allUsers"
}

output "adminapi_url" {
  value = google_cloud_run_service.adminapi.status.0.url
}
