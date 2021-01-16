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

resource "google_service_account" "docker-mirror" {
  project      = var.project
  account_id   = "docker-mirror"
  display_name = "Docker Hub Mirror"
}

resource "google_artifact_registry_repository_iam_member" "mirror-cloud-run-writer" {
  provider = google-beta

  project    = google_artifact_registry_repository.mirrors.project
  location   = google_artifact_registry_repository.mirrors.location
  repository = google_artifact_registry_repository.mirrors.name

  member = "serviceAccount:${google_service_account.docker-mirror.email}"
  role   = "roles/artifactregistry.writer"
}

resource "google_cloud_run_service" "docker-mirror" {
  name     = "docker-mirror"
  location = var.region

  autogenerate_revision_name = true

  template {
    spec {
      service_account_name = google_service_account.docker-mirror.email
      timeout_seconds      = 900

      containers {
        image = "us-docker.pkg.dev/vargolabs/cloud-run-docker-mirror/server:latest"

        resources {
          limits = {
            cpu    = "2"
            memory = "1Gi"
          }
        }
      }
    }
  }

  depends_on = [
    google_project_service.services["run.googleapis.com"],
  ]
}

resource "google_service_account" "docker-mirror-invoker" {
  project      = data.google_project.project.project_id
  account_id   = "docker-mirror-invoker"
  display_name = "Docker Mirror Invoker"
}

resource "google_cloud_run_service_iam_member" "docker-mirror-invoker" {
  project  = google_cloud_run_service.docker-mirror.project
  location = google_cloud_run_service.docker-mirror.location
  service  = google_cloud_run_service.docker-mirror.name
  role     = "roles/run.invoker"
  member   = "serviceAccount:${google_service_account.docker-mirror-invoker.email}"
}

resource "google_cloud_scheduler_job" "docker-mirror-worker" {
  name             = "docker-mirror-worker"
  region           = var.cloudscheduler_location
  schedule         = "0 11 * * *"
  time_zone        = "America/Los_Angeles"
  attempt_deadline = "900s"

  http_target {
    http_method = "POST"
    uri         = "${google_cloud_run_service.docker-mirror.status.0.url}/"

    body = base64encode(jsonencode({
      mirrors = [
        {
          src = "postgres:13"
          dst = "${google_artifact_registry_repository.mirrors.location}-docker.pkg.dev/${var.project}/mirrors/postgres:13"
        },
        {
          src = "postgres:13-alpine"
          dst = "${google_artifact_registry_repository.mirrors.location}-docker.pkg.dev/${var.project}/mirrors/postgres:13-alpine"
        },
        {
          src = "redis:6"
          dst = "${google_artifact_registry_repository.mirrors.location}-docker.pkg.dev/${var.project}/mirrors/redis:6"
        },
        {
          src = "redis:6-alpine"
          dst = "${google_artifact_registry_repository.mirrors.location}-docker.pkg.dev/${var.project}/mirrors/redis:6-alpine"
        },
      ]
    }))

    oidc_token {
      audience              = google_cloud_run_service.docker-mirror.status.0.url
      service_account_email = google_service_account.docker-mirror-invoker.email
    }
  }

  depends_on = [
    # This also depends on AppEngine, but we can't declare that dependency here
    # because it was actually created upstream.

    google_cloud_run_service_iam_member.docker-mirror-invoker,
    google_project_service.services["cloudscheduler.googleapis.com"],
    google_artifact_registry_repository_iam_member.mirror-cloud-run-writer,
  ]
}
