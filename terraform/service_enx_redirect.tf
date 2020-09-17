
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

resource "google_service_account" "redirect" {
  project      = var.project
  account_id   = "en-verification-redirect-sa"
  display_name = "Verification redirect"
}

resource "google_service_account_iam_member" "cloudbuild-deploy-redirect" {
  service_account_id = google_service_account.redirect.id
  role               = "roles/iam.serviceAccountUser"
  member             = "serviceAccount:${data.google_project.project.number}@cloudbuild.gserviceaccount.com"

  depends_on = [
    google_project_service.services["cloudbuild.googleapis.com"],
    google_project_service.services["iam.googleapis.com"],
  ]
}

resource "google_project_iam_member" "redirect-observability" {
  for_each = toset([
    "roles/cloudtrace.agent",
    "roles/logging.logWriter",
    "roles/monitoring.metricWriter",
    "roles/stackdriver.resourceMetadata.writer",
  ])

  project = var.project
  role    = each.key
  member  = "serviceAccount:${google_service_account.redirect.email}"
}

resource "google_cloud_run_service" "redirect" {
  name     = "redirect"
  location = var.region

  autogenerate_revision_name = true

  template {
    spec {
      service_account_name = google_service_account.redirect.email
      timeout_seconds      = 25

      containers {
        image = "gcr.io/${var.project}/github.com/google/exposure-notifications-verification-redirect/redirect:initial"

        resources {
          limits = {
            cpu    = "1"
            memory = "512Mi"
          }
        }

        dynamic "env" {
          for_each = merge(
            local.cache_config,
            local.csrf_config,
            local.gcp_config,
            local.rate_limit_config,
            local.session_config,
            local.redirect_config,

            // This MUST come last to allow overrides!
            lookup(var.service_environment, "redirect", {}),
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
    google_secret_manager_secret_iam_member.redirect-db,
    null_resource.build,
  ]

  lifecycle {
    ignore_changes = [
      template[0].metadata[0].annotations,
      template[0].spec[0].containers[0].image,
    ]
  }
}

resource "google_compute_region_network_endpoint_group" "redirect" {
  name     = "redirect"
  provider = google-beta
  project  = var.project
  region   = var.region

  network_endpoint_type = "SERVERLESS"

  cloud_run {
    service = google_cloud_run_service.redirect.name
  }
}

resource "google_compute_backend_service" "redirect" {
  count    = local.enable_lb ? 1 : 0
  provider = google-beta
  name     = "redirect"
  project  = var.project

  backend {
    group = google_compute_region_network_endpoint_group.redirect.id
  }
}

resource "google_cloud_run_domain_mapping" "redirect" {
  for_each = var.redirect_custom_domains

  location = var.cloudrun_location
  name     = each.key

  metadata {
    namespace = var.project
  }

  spec {
    route_name     = google_cloud_run_service.redirect.name
    force_override = true
  }

  lifecycle {
    ignore_changes = [
      spec[0].force_override
    ]
  }
}

resource "google_cloud_run_service_iam_member" "redirect-public" {
  location = google_cloud_run_service.redirect.location
  project  = google_cloud_run_service.redirect.project
  service  = google_cloud_run_service.redirect.name
  role     = "roles/run.invoker"
  member   = "allUsers"
}

output "redirect_url" {
  value = google_cloud_run_service.redirect.status.0.url
}
