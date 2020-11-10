
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

resource "google_service_account" "enx-redirect" {
  project = var.project
  # TODO(sethvargo): namespace this, but SA are limited to 28 characters :(
  account_id   = "enx-redirect-sa"
  display_name = "Verification enx-redirect"
}

resource "google_service_account_iam_member" "cloudbuild-deploy-enx-redirect" {
  service_account_id = google_service_account.enx-redirect.id
  role               = "roles/iam.serviceAccountUser"
  member             = "serviceAccount:${data.google_project.project.number}@cloudbuild.gserviceaccount.com"

  depends_on = [
    google_project_service.services["cloudbuild.googleapis.com"],
    google_project_service.services["iam.googleapis.com"],
  ]
}

resource "google_secret_manager_secret_iam_member" "enx-redirect-db" {
  for_each = toset([
    "sslcert",
    "sslkey",
    "sslrootcert",
    "password",
  ])

  secret_id = google_secret_manager_secret.db-secret[each.key].id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.enx-redirect.email}"
}

resource "google_secret_manager_secret_iam_member" "enx-redirect-csrf" {
  secret_id = google_secret_manager_secret.csrf-token.id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.enx-redirect.email}"
}

resource "google_secret_manager_secret_iam_member" "enx-redirect-cookie-hmac-key" {
  provider  = google-beta
  secret_id = google_secret_manager_secret.cookie-hmac-key.id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.enx-redirect.email}"
}

resource "google_kms_key_ring_iam_member" "enx-redirect-verification-key-admin" {
  key_ring_id = google_kms_key_ring.verification.self_link
  role        = "roles/cloudkms.admin"
  member      = "serviceAccount:${google_service_account.enx-redirect.email}"
}

resource "google_kms_key_ring_iam_member" "enx-redirect-verification-key-signer-verifier" {
  key_ring_id = google_kms_key_ring.verification.self_link
  role        = "roles/cloudkms.signerVerifier"
  member      = "serviceAccount:${google_service_account.enx-redirect.email}"
}

resource "google_kms_crypto_key_iam_member" "enx-redirect-database-encrypter" {
  crypto_key_id = google_kms_crypto_key.database-encrypter.self_link
  role          = "roles/cloudkms.cryptoKeyEncrypterDecrypter"
  member        = "serviceAccount:${google_service_account.enx-redirect.email}"
}

resource "google_secret_manager_secret_iam_member" "enx-redirect-db-apikey-db-hmac" {
  secret_id = google_secret_manager_secret.db-apikey-db-hmac.id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.enx-redirect.email}"
}

resource "google_secret_manager_secret_iam_member" "enx-redirect-db-apikey-sig-hmac" {
  secret_id = google_secret_manager_secret.db-apikey-sig-hmac.id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.enx-redirect.email}"
}

resource "google_secret_manager_secret_iam_member" "enx-redirect-db-verification-code-hmac" {
  secret_id = google_secret_manager_secret.db-verification-code-hmac.id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.enx-redirect.email}"
}

resource "google_secret_manager_secret_iam_member" "enx-redirect-cache-hmac-key" {
  secret_id = google_secret_manager_secret.cache-hmac-key.id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.enx-redirect.email}"
}

resource "google_project_iam_member" "enx-redirect-observability" {
  for_each = toset([
    "roles/cloudtrace.agent",
    "roles/logging.logWriter",
    "roles/monitoring.metricWriter",
    "roles/stackdriver.resourceMetadata.writer",
  ])

  project = var.project
  role    = each.key
  member  = "serviceAccount:${google_service_account.enx-redirect.email}"
}

resource "google_cloud_run_service" "enx-redirect" {
  name     = "enx-redirect"
  location = var.region

  autogenerate_revision_name = true

  template {
    spec {
      service_account_name = google_service_account.enx-redirect.email
      timeout_seconds      = 25

      containers {
        image = "gcr.io/${var.project}/github.com/google/exposure-notifications-verification-server/enx-redirect:initial"

        resources {
          limits = {
            cpu    = "1"
            memory = "512Mi"
          }
        }

        dynamic "env" {
          for_each = merge(
            local.gcp_config,
            local.enx_redirect_config,
            local.cache_config,
            local.database_config,

            // This MUST come last to allow overrides!
            lookup(var.service_environment, "enx-redirect", {}),
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

    google_secret_manager_secret_iam_member.enx-redirect-db,
    google_secret_manager_secret_iam_member.enx-redirect-csrf,
    google_secret_manager_secret_iam_member.enx-redirect-cookie-hmac-key,
    google_kms_key_ring_iam_member.enx-redirect-verification-key-admin,
    google_kms_key_ring_iam_member.enx-redirect-verification-key-signer-verifier,
    google_kms_crypto_key_iam_member.enx-redirect-database-encrypter,
    google_secret_manager_secret_iam_member.enx-redirect-db-apikey-db-hmac,
    google_secret_manager_secret_iam_member.enx-redirect-db-apikey-sig-hmac,
    google_secret_manager_secret_iam_member.enx-redirect-db-verification-code-hmac,
    google_secret_manager_secret_iam_member.enx-redirect-cache-hmac-key,
    google_project_iam_member.enx-redirect-observability,

    null_resource.build,
    null_resource.migrate,
  ]

  lifecycle {
    ignore_changes = [
      template[0].metadata[0].annotations,
      template[0].spec[0].containers[0].image,
    ]
  }
}

resource "google_compute_region_network_endpoint_group" "enx-redirect" {
  name     = "enx-redirect"
  provider = google-beta
  project  = var.project
  region   = var.region

  network_endpoint_type = "SERVERLESS"

  cloud_run {
    service = google_cloud_run_service.enx-redirect.name
  }
}

resource "google_compute_backend_service" "enx-redirect" {
  count    = local.enable_lb ? 1 : 0
  provider = google-beta
  name     = "enx-redirect"
  project  = var.project

  backend {
    group = google_compute_region_network_endpoint_group.enx-redirect.id
  }
}

resource "google_cloud_run_service_iam_member" "enx-redirect-public" {
  location = google_cloud_run_service.enx-redirect.location
  project  = google_cloud_run_service.enx-redirect.project
  service  = google_cloud_run_service.enx-redirect.name
  role     = "roles/run.invoker"
  member   = "allUsers"
}

output "enx_redirect_url" {
  value = google_cloud_run_service.enx-redirect.status.0.url
}
