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

resource "google_service_account" "modeler" {
  project      = var.project
  account_id   = "en-verification-modeler-sa"
  display_name = "Verification modeler"
}

resource "google_service_account_iam_member" "cloudbuild-deploy-modeler" {
  service_account_id = google_service_account.modeler.id
  role               = "roles/iam.serviceAccountUser"
  member             = "serviceAccount:${local.cloudbuild_email}"
}

resource "google_project_iam_member" "modeler-observability" {
  for_each = local.observability_iam_roles
  project  = var.project
  role     = each.key
  member   = "serviceAccount:${google_service_account.modeler.email}"
}

resource "google_kms_crypto_key_iam_member" "modeler-database-encrypter" {
  crypto_key_id = google_kms_crypto_key.database-encrypter.self_link
  role          = "roles/cloudkms.cryptoKeyEncrypterDecrypter"
  member        = "serviceAccount:${google_service_account.modeler.email}"
}

locals {
  modeler_secrets = flatten([
    local.database_secrets,
    local.redis_secrets,
  ])
}

resource "google_secret_manager_secret_iam_member" "modeler-secrets" {
  count     = length(local.modeler_secrets)
  secret_id = element(local.modeler_secrets, count.index)
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.modeler.email}"
}

resource "google_cloud_run_service" "modeler" {
  name     = "modeler"
  location = var.region

  autogenerate_revision_name = true

  metadata {
    annotations = merge(
      local.default_service_annotations,
      var.default_service_annotations_overrides,
      lookup(var.service_annotations, "modeler", {})
    )
  }
  template {
    spec {
      service_account_name = google_service_account.modeler.email

      containers {
        image = "gcr.io/${var.project}/github.com/google/exposure-notifications-verification-server/modeler:initial"

        resources {
          limits = {
            cpu    = "2"
            memory = "512Mi"
          }
        }

        dynamic "env" {
          for_each = merge(
            local.cache_config,
            local.database_config,
            local.gcp_config,
            local.rate_limit_config,
            local.observability_config,

            // This MUST come last to allow overrides!
            lookup(var.service_environment, "modeler", {}),
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
        lookup(var.revision_annotations, "modeler", {})
      )
    }
  }

  depends_on = [
    google_project_service.services["run.googleapis.com"],

    google_kms_crypto_key_iam_member.modeler-database-encrypter,
    google_project_iam_member.modeler-observability,
    google_secret_manager_secret_iam_member.modeler-secrets,
    google_service_account_iam_member.cloudbuild-deploy-modeler,

    null_resource.build,
    null_resource.migrate,
  ]

  lifecycle {
    ignore_changes = [
      metadata[0].annotations["client.knative.dev/user-image"],
      metadata[0].annotations["run.googleapis.com/client-name"],
      metadata[0].annotations["run.googleapis.com/client-version"],
      metadata[0].annotations["run.googleapis.com/ingress-status"],
      metadata[0].annotations["serving.knative.dev/creator"],
      metadata[0].annotations["serving.knative.dev/lastModifier"],
      metadata[0].labels["cloud.googleapis.com/location"],
      template[0].metadata[0].annotations["client.knative.dev/user-image"],
      template[0].metadata[0].annotations["run.googleapis.com/client-name"],
      template[0].metadata[0].annotations["run.googleapis.com/client-version"],
      template[0].metadata[0].annotations["serving.knative.dev/creator"],
      template[0].metadata[0].annotations["serving.knative.dev/lastModifier"],
      template[0].spec[0].containers[0].image,
    ]
  }
}

resource "google_compute_region_network_endpoint_group" "modeler" {
  name     = "modeler"
  provider = google-beta
  project  = var.project
  region   = var.region

  network_endpoint_type = "SERVERLESS"

  cloud_run {
    service = google_cloud_run_service.modeler.name
  }
}

resource "google_compute_backend_service" "modeler" {
  count    = local.enable_lb ? 1 : 0
  provider = google-beta
  name     = "modeler"
  project  = var.project

  backend {
    group = google_compute_region_network_endpoint_group.modeler.id
  }
  security_policy = google_compute_security_policy.cloud-armor.name
  log_config {
    enable      = var.enable_lb_logging
    sample_rate = var.enable_lb_logging ? 1 : null
  }
}

output "modeler_url" {
  value = google_cloud_run_service.modeler.status.0.url
}

#
# Create scheduler job to invoke the service on a fixed interval.
#

resource "google_service_account" "modeler-invoker" {
  project      = data.google_project.project.project_id
  account_id   = "en-modeler-invoker-sa"
  display_name = "Verification modeler invoker"
}

resource "google_cloud_run_service_iam_member" "modeler-invoker" {
  project  = google_cloud_run_service.modeler.project
  location = google_cloud_run_service.modeler.location
  service  = google_cloud_run_service.modeler.name
  role     = "roles/run.invoker"
  member   = "serviceAccount:${google_service_account.modeler-invoker.email}"
}

resource "google_cloud_scheduler_job" "modeler-worker" {
  name             = "modeler-worker"
  region           = var.cloudscheduler_location
  schedule         = "0 */6 * * *"
  time_zone        = "America/Los_Angeles"
  attempt_deadline = "600s"

  retry_config {
    retry_count = 3
  }

  http_target {
    http_method = "POST"
    uri         = "${google_cloud_run_service.modeler.status.0.url}/"
    oidc_token {
      audience              = google_cloud_run_service.modeler.status.0.url
      service_account_email = google_service_account.modeler-invoker.email
    }
  }

  depends_on = [
    google_app_engine_application.app,
    google_cloud_run_service_iam_member.modeler-invoker,
    google_project_service.services["cloudscheduler.googleapis.com"],
  ]
}
