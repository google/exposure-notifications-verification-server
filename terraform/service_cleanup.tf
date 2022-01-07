# Copyright 2020 the Exposure Notifications Verification Server authors
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
  member             = "serviceAccount:${local.cloudbuild_email}"

  depends_on = [
    google_project_service.services["cloudbuild.googleapis.com"],
  ]
}

resource "google_project_iam_member" "cleanup-observability" {
  for_each = local.observability_iam_roles
  project  = var.project
  role     = each.key
  member   = "serviceAccount:${google_service_account.cleanup.email}"
}

resource "google_kms_crypto_key_iam_member" "cleanup-database-encrypter" {
  crypto_key_id = google_kms_crypto_key.database-encrypter.self_link
  role          = "roles/cloudkms.cryptoKeyEncrypterDecrypter"
  member        = "serviceAccount:${google_service_account.cleanup.email}"
}

resource "google_kms_crypto_key_iam_member" "cleanup-cert-signing-admin" {
  crypto_key_id = google_kms_crypto_key.certificate-signer.self_link
  role          = "roles/cloudkms.admin"
  member        = "serviceAccount:${google_service_account.cleanup.email}"
}

resource "google_kms_crypto_key_iam_member" "cleanup-token-signing-admin" {
  crypto_key_id = google_kms_crypto_key.token-signer.self_link
  role          = "roles/cloudkms.admin"
  member        = "serviceAccount:${google_service_account.cleanup.email}"
}

locals {
  cleanup_secrets = flatten([
    local.database_secrets,
    local.redis_secrets,
  ])
}

resource "google_secret_manager_secret_iam_member" "cleanup-secrets" {
  count     = length(local.cleanup_secrets)
  secret_id = element(local.cleanup_secrets, count.index)
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.cleanup.email}"
}

resource "google_cloud_run_service" "cleanup" {
  name     = "cleanup"
  location = var.region

  autogenerate_revision_name = true

  metadata {
    annotations = merge(
      local.default_service_annotations,
      var.default_service_annotations_overrides,
      lookup(var.service_annotations, "cleanup", {})
    )
  }

  template {
    spec {
      service_account_name = google_service_account.cleanup.email
      timeout_seconds      = 900

      containers {
        image = "gcr.io/${var.project}/github.com/google/exposure-notifications-verification-server/cleanup:initial"

        resources {
          limits = {
            cpu    = "1"
            memory = "512Mi"
          }
        }


        dynamic "env" {
          for_each = merge(
            local.cache_config,
            local.database_config,
            local.feature_config,
            local.firebase_config,
            local.gcp_config,
            local.signing_config,
            local.observability_config,

            // This MUST come last to allow overrides!
            lookup(var.service_environment, "_all", {}),
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
      annotations = merge(
        local.default_revision_annotations,
        var.default_revision_annotations_overrides,
        lookup(var.revision_annotations, "cleanup", {})
      )
    }
  }

  depends_on = [
    google_project_service.services["run.googleapis.com"],

    google_kms_crypto_key_iam_member.cleanup-cert-signing-admin,
    google_kms_crypto_key_iam_member.cleanup-database-encrypter,
    google_kms_crypto_key_iam_member.cleanup-token-signing-admin,
    google_project_iam_member.cleanup-observability,
    google_secret_manager_secret_iam_member.cleanup-secrets,

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
  schedule         = "*/5 * * * *"
  time_zone        = var.cloud_scheduler_timezone
  attempt_deadline = "${google_cloud_run_service.cleanup.template[0].spec[0].timeout_seconds + 60}s"

  retry_config {
    retry_count = 3
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
