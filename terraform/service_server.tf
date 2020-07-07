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
  provider = google-beta

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
  provider  = google-beta
  secret_id = google_secret_manager_secret.csrf-token.id
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

resource "google_cloud_run_service" "server" {
  name     = "server"
  location = var.region

  template {
    spec {
      service_account_name = google_service_account.server.email

      containers {
        image = "gcr.io/${var.project}/github.com/google/exposure-notifications-verification-server/cmd/server:initial"

        resources {
          limits = {
            cpu    = "1"
            memory = "512Mi"
          }
        }

        dynamic "env" {
          for_each = local.gcp_config
          content {
            name  = env.key
            value = env.value
          }
        }

        dynamic "env" {
          for_each = {
            # Assets - these are built into the container
            ASSETS_PATH = "/assets"
          }

          content {
            name  = env.key
            value = env.value
          }
        }

        dynamic "env" {
          for_each = local.csrf_config
          content {
            name  = env.key
            value = env.value
          }
        }

        dynamic "env" {
          for_each = local.database_config
          content {
            name  = env.key
            value = env.value
          }
        }

        dynamic "env" {
          for_each = local.firebase_config
          content {
            name  = env.key
            value = env.value
          }
        }

        dynamic "env" {
          for_each = local.signing_config
          content {
            name  = env.key
            value = env.value
          }
        }

        dynamic "env" {
          for_each = lookup(var.service_environment, "server", {})
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
    google_secret_manager_secret_iam_member.server-db,
    null_resource.build,
  ]

  lifecycle {
    ignore_changes = [
      template,
    ]
  }
}

resource "google_cloud_run_service_iam_member" "server-public" {
  location = google_cloud_run_service.server.location
  project  = google_cloud_run_service.server.project
  service  = google_cloud_run_service.server.name
  role     = "roles/run.invoker"
  member   = "allUsers"
}

output "server_url" {
  value = google_cloud_run_service.server.status.0.url
}
