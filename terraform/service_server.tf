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

resource "google_service_account" "server" {
  project      = var.project
  account_id   = "en-verification-server-sa"
  display_name = "Verification server"
}

resource "google_service_account_iam_member" "cloudbuild-deploy-server" {
  service_account_id = google_service_account.server.id
  role               = "roles/iam.serviceAccountUser"
  member             = "serviceAccount:${data.google_project.project.number}@cloudbuild.gserviceaccount.com"

  depends_on = [
    google_project_service.services["cloudbuild.googleapis.com"],
    google_project_service.services["iam.googleapis.com"],
  ]
}

resource "google_secret_manager_secret_iam_member" "server-db" {
  for_each = toset([
    "sslcert",
    "sslkey",
    "sslrootcert",
    "password",
  ])

  secret_id = google_secret_manager_secret.db-secret[each.key].id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.server.email}"
}

resource "google_secret_manager_secret_iam_member" "server-csrf" {
  secret_id = google_secret_manager_secret.csrf-token.id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.server.email}"
}

resource "google_secret_manager_secret_iam_member" "server-cookie-hmac-key" {
  provider  = google-beta
  secret_id = google_secret_manager_secret.cookie-hmac-key.id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.server.email}"
}

resource "google_secret_manager_secret_iam_member" "server-cookie-encryption-key" {
  secret_id = google_secret_manager_secret.cookie-encryption-key.id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.server.email}"
}

resource "google_project_iam_member" "firebase-admin" {
  project = var.project
  role    = "roles/firebaseauth.admin"
  member  = "serviceAccount:${google_service_account.server.email}"
}

resource "google_project_iam_member" "server-observability" {
  for_each = toset([
    "roles/cloudtrace.agent",
    "roles/logging.logWriter",
    "roles/monitoring.metricWriter",
    "roles/stackdriver.resourceMetadata.writer",
  ])

  project = var.project
  role    = each.key
  member  = "serviceAccount:${google_service_account.server.email}"
}

resource "google_kms_key_ring_iam_member" "server-verification-key-admin" {
  key_ring_id = google_kms_key_ring.verification.self_link
  role        = "roles/cloudkms.admin"
  member      = "serviceAccount:${google_service_account.server.email}"
}

resource "google_kms_key_ring_iam_member" "server-verification-key-signer-verifier" {
  key_ring_id = google_kms_key_ring.verification.self_link
  role        = "roles/cloudkms.signerVerifier"
  member      = "serviceAccount:${google_service_account.server.email}"
}

resource "google_kms_crypto_key_iam_member" "server-database-encrypter" {
  crypto_key_id = google_kms_crypto_key.database-encrypter.self_link
  role          = "roles/cloudkms.cryptoKeyEncrypterDecrypter"
  member        = "serviceAccount:${google_service_account.server.email}"
}

resource "google_secret_manager_secret_iam_member" "server-db-apikey-db-hmac" {
  secret_id = google_secret_manager_secret.db-apikey-db-hmac.id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.server.email}"
}

resource "google_secret_manager_secret_iam_member" "server-db-apikey-sig-hmac" {
  secret_id = google_secret_manager_secret.db-apikey-sig-hmac.id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.server.email}"
}

resource "google_secret_manager_secret_iam_member" "server-db-verification-code-hmac" {
  secret_id = google_secret_manager_secret.db-verification-code-hmac.id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.server.email}"
}

resource "google_secret_manager_secret_iam_member" "server-cache-hmac-key" {
  secret_id = google_secret_manager_secret.cache-hmac-key.id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.server.email}"
}

resource "google_secret_manager_secret_iam_member" "server-ratelimit-hmac-key" {
  secret_id = google_secret_manager_secret.ratelimit-hmac-key.id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.server.email}"
}

resource "google_cloud_run_service" "server" {
  name     = "server"
  location = var.region

  autogenerate_revision_name = true

  template {
    spec {
      service_account_name = google_service_account.server.email
      timeout_seconds      = 25

      containers {
        image = "gcr.io/${var.project}/github.com/google/exposure-notifications-verification-server/server:initial"

        resources {
          limits = {
            cpu    = "1"
            memory = "512Mi"
          }
        }

        dynamic "env" {
          for_each = merge(
            { "ASSETS_PATH" = "/assets" },
            local.cache_config,
            local.csrf_config,
            local.database_config,
            local.firebase_config,
            local.gcp_config,
            local.rate_limit_config,
            local.session_config,
            local.signing_config,
            local.issue_config,

            // This MUST come last to allow overrides!
            lookup(var.service_environment, "server", {}),
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

    google_kms_crypto_key_iam_member.server-database-encrypter,
    google_kms_key_ring_iam_member.server-verification-key-admin,
    google_kms_key_ring_iam_member.server-verification-key-signer-verifier,
    google_project_iam_member.firebase-admin,
    google_project_iam_member.server-observability,
    google_secret_manager_secret_iam_member.server-cache-hmac-key,
    google_secret_manager_secret_iam_member.server-db-apikey-db-hmac,
    google_secret_manager_secret_iam_member.server-db-apikey-sig-hmac,
    google_secret_manager_secret_iam_member.server-db-verification-code-hmac,
    google_secret_manager_secret_iam_member.server-ratelimit-hmac-key,
    google_secret_manager_secret_iam_member.server-cookie-encryption-key,
    google_secret_manager_secret_iam_member.server-cookie-hmac-key,
    google_secret_manager_secret_iam_member.server-csrf,
    google_secret_manager_secret_iam_member.server-db,

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

resource "google_compute_region_network_endpoint_group" "server" {
  count = length(var.server_hosts) > 0 ? 1 : 0

  name     = "server"
  provider = google-beta
  project  = var.project
  region   = var.region

  network_endpoint_type = "SERVERLESS"

  cloud_run {
    service = google_cloud_run_service.server.name
  }
}

resource "google_compute_backend_service" "server" {
  count = length(var.server_hosts) > 0 ? 1 : 0

  provider = google-beta
  name     = "server"
  project  = var.project

  backend {
    group = google_compute_region_network_endpoint_group.server[0].id
  }
}

resource "google_cloud_run_service_iam_member" "server-public" {
  location = google_cloud_run_service.server.location
  project  = google_cloud_run_service.server.project
  service  = google_cloud_run_service.server.name
  role     = "roles/run.invoker"
  member   = "allUsers"
}

output "server_urls" {
  value = concat([google_cloud_run_service.server.status.0.url], formatlist("https://%s", var.server_hosts))
}
