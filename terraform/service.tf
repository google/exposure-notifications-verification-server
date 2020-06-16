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
  common_envvars = [
    {
      name  = "DB_SSLMODE"
      value = "verify-ca"
    },
    {
      name  = "DB_HOST"
      value = google_sql_database_instance.db-inst.private_ip_address
    },
    {
      name  = "DB_NAME"
      value = google_sql_database.db.name
    },
    {
      name  = "DB_SSLCERT"
      value = "secret://${google_secret_manager_secret_version.db-secret-version["sslcert"].id}?target=file"
    },
    {
      name  = "DB_SSLKEY"
      value = "secret://${google_secret_manager_secret_version.db-secret-version["sslkey"].id}?target=file"
    },
    {
      name  = "DB_SSLROOTCERT"
      value = "secret://${google_secret_manager_secret_version.db-secret-version["sslrootcert"].id}?target=file"
    },
    {
      name  = "DB_USER"
      value = google_sql_user.user.name
    },
    {
      name  = "DB_PASSWORD"
      value = "secret://${google_secret_manager_secret_version.db-secret-version["password"].id}"
    },
  ]
}

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
          for_each = local.common_envvars
          content {
            name  = env.value["name"]
            value = env.value["value"]
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
