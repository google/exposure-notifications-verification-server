
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

resource "google_service_account" "enx-redirect" {
  project      = var.project
  account_id   = "enx-redirect-sa"
  display_name = "Verification enx-redirect"
}

resource "google_service_account_iam_member" "cloudbuild-deploy-enx-redirect" {
  service_account_id = google_service_account.enx-redirect.id
  role               = "roles/iam.serviceAccountUser"
  member             = "serviceAccount:${local.cloudbuild_email}"
}

resource "google_project_iam_member" "enx-redirect-observability" {
  for_each = local.observability_iam_roles
  project  = var.project
  role     = each.key
  member   = "serviceAccount:${google_service_account.enx-redirect.email}"
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

locals {
  enx_redirect_secrets = flatten([
    local.database_secrets,
    local.redis_secrets,
    local.session_secrets,
  ])
}

resource "google_secret_manager_secret_iam_member" "enx-redirect-secrets" {
  count     = length(local.enx_redirect_secrets)
  secret_id = element(local.enx_redirect_secrets, count.index)
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.enx-redirect.email}"
}

resource "google_cloud_run_service" "enx-redirect" {
  name     = "enx-redirect"
  location = var.region

  autogenerate_revision_name = true

  metadata {
    annotations = merge(
      local.default_service_annotations,
      var.default_service_annotations_overrides,
      lookup(var.service_annotations, "enx-redirect", {})
    )
  }
  template {
    spec {
      service_account_name = google_service_account.enx-redirect.email
      timeout_seconds      = 10

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
            local.observability_config,
            local.rate_limit_config,
            local.signing_config,
            local.issue_config,
            local.session_config,

            // This MUST come last to allow overrides!
            lookup(var.service_environment, "_all", {}),
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
      annotations = merge(
        local.default_revision_annotations,
        var.default_revision_annotations_overrides,
        lookup(var.revision_annotations, "enx-redirect", {})
      )
    }
  }

  depends_on = [
    google_project_service.services["run.googleapis.com"],

    google_kms_crypto_key_iam_member.enx-redirect-database-encrypter,
    google_kms_key_ring_iam_member.enx-redirect-verification-key-admin,
    google_kms_key_ring_iam_member.enx-redirect-verification-key-signer-verifier,
    google_project_iam_member.enx-redirect-observability,
    google_secret_manager_secret_iam_member.enx-redirect-secrets,

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
  count = local.enable_lb ? 1 : 0

  provider = google-beta
  name     = "enx-redirect"
  project  = var.project

  security_policy = google_compute_security_policy.cloud-armor.name

  backend {
    group = google_compute_region_network_endpoint_group.enx-redirect.id
  }

  log_config {
    enable      = var.enable_lb_logging
    sample_rate = var.enable_lb_logging ? 1 : null
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
