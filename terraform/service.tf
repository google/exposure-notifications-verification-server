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

resource "google_service_account" "verification" {
  project      = var.project
  account_id   = "en-verification-sa"
  display_name = "Verification"
}

resource "google_service_account_iam_member" "cloudbuild-deploy-verification" {
  service_account_id = google_service_account.verification.id
  role               = "roles/iam.serviceAccountUser"
  member             = "serviceAccount:${data.google_project.project.number}@cloudbuild.gserviceaccount.com"

  depends_on = [
    google_project_service.services["cloudbuild.googleapis.com"],
    google_project_service.services["iam.googleapis.com"],
  ]
}

resource "google_secret_manager_secret_iam_member" "verification-db" {
  provider = google-beta

  for_each = toset([
    "sslcert",
    "sslkey",
    "sslrootcert",
    "password",
  ])

  secret_id = google_secret_manager_secret.db-secret[each.key].id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.verification.email}"
}

resource "google_cloud_run_service" "verification" {
  name     = "verification"
  location = var.region

  template {
    spec {
      service_account_name = google_service_account.verification.email

      containers {
        image = "gcr.io/${var.project}/github.com/google/exposure-notifications-verification-server/cmd/server:initial"

        resources {
          limits = {
            cpu    = "1"
            memory = "512Mi"
          }
        }

        dynamic "env" {
          for_each = {
            # Assets - these are built into the container
            ASSETS_PATH = "/assets"

            # Database
            DB_HOST        = google_sql_database_instance.db-inst.private_ip_address
            DB_NAME        = google_sql_database.db.name
            DB_PASSWORD    = "secret://${google_secret_manager_secret_version.db-secret-version["password"].id}"
            DB_SSLCERT     = "secret://${google_secret_manager_secret_version.db-secret-version["sslcert"].id}?target=file"
            DB_SSLKEY      = "secret://${google_secret_manager_secret_version.db-secret-version["sslkey"].id}?target=file"
            DB_SSLMODE     = "verify-ca"
            DB_SSLROOTCERT = "secret://${google_secret_manager_secret_version.db-secret-version["sslrootcert"].id}?target=file"
            DB_USER        = google_sql_user.user.name

            # Firebase
            FIREBASE_API_KEY           = data.google_firebase_web_app_config.default.api_key
            FIREBASE_APP_ID            = google_firebase_web_app.default.app_id
            FIREBASE_AUTH_DOMAIN       = data.google_firebase_web_app_config.default.auth_domain
            FIREBASE_DATABASE_URL      = lookup(data.google_firebase_web_app_config.default, "database_url")
            FIREBASE_MEASUREMENT_ID    = lookup(data.google_firebase_web_app_config.default, "measurement_id")
            FIREBASE_MESSAGE_SENDER_ID = lookup(data.google_firebase_web_app_config.default, "messaging_sender_id")
            FIREBASE_PROJECT_ID        = google_firebase_web_app.default.project
            FIREBASE_STORAGE_BUCKET    = lookup(data.google_firebase_web_app_config.default, "storage_bucket")

            # Signing
            CERTIFICATE_SIGNING_KEY = google_kms_crypto_key.certificate-signer.self_link
            TOKEN_SIGNING_KEY       = google_kms_crypto_key.token-signer.self_link
          }

          content {
            name  = env.key
            value = env.value
          }
        }

        dynamic "env" {
          for_each = lookup(var.service_environment, "verification", {})
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
    google_secret_manager_secret_iam_member.verification-db,
    null_resource.build,
  ]

  lifecycle {
    ignore_changes = [
      template,
    ]
  }
}
