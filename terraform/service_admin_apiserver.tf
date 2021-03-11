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

resource "google_service_account" "adminapi" {
  project      = var.project
  account_id   = "en-verification-adminapi-sa"
  display_name = "Verification Admin apiserver"
}

resource "google_service_account_iam_member" "cloudbuild-deploy-adminapi" {
  service_account_id = google_service_account.adminapi.id
  role               = "roles/iam.serviceAccountUser"
  member             = "serviceAccount:${local.cloudbuild_email}"
}

resource "google_project_iam_member" "adminapi-observability" {
  for_each = local.observability_iam_roles
  project  = var.project
  role     = each.key
  member   = "serviceAccount:${google_service_account.adminapi.email}"
}

resource "google_kms_key_ring_iam_member" "adminapi-verification-signer-verifier" {
  key_ring_id = google_kms_key_ring.verification.self_link
  role        = "roles/cloudkms.signerVerifier"
  member      = "serviceAccount:${google_service_account.adminapi.email}"
}

resource "google_kms_crypto_key_iam_member" "adminapi-database-encrypter" {
  crypto_key_id = google_kms_crypto_key.database-encrypter.self_link
  role          = "roles/cloudkms.cryptoKeyEncrypterDecrypter"
  member        = "serviceAccount:${google_service_account.adminapi.email}"
}

locals {
  adminapi_secrets = flatten([
    local.database_secrets,
    local.redis_secrets,
  ])
}

resource "google_secret_manager_secret_iam_member" "adminapi-secrets" {
  count     = length(local.adminapi_secrets)
  secret_id = element(local.adminapi_secrets, count.index)
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.adminapi.email}"
}

resource "google_cloud_run_service" "adminapi" {
  name     = "adminapi"
  location = var.region

  autogenerate_revision_name = true

  metadata {
    annotations = merge(
      local.default_service_annotations,
      var.default_service_annotations_overrides,
      lookup(var.service_annotations, "adminapi", {})
    )
  }

  template {
    spec {
      service_account_name = google_service_account.adminapi.email
      timeout_seconds      = 25

      containers {
        image = "gcr.io/${var.project}/github.com/google/exposure-notifications-verification-server/adminapi:initial"

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
            local.gcp_config,
            local.rate_limit_config,
            local.issue_config,
            local.signing_config,
            local.observability_config,

            // This MUST come last to allow overrides!
            lookup(var.service_environment, "adminapi", {}),
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
        lookup(var.revision_annotations, "adminapi", {})
      )
    }
  }

  depends_on = [
    google_project_service.services["run.googleapis.com"],

    google_kms_crypto_key_iam_member.adminapi-database-encrypter,
    google_kms_key_ring_iam_member.adminapi-verification-signer-verifier,
    google_project_iam_member.adminapi-observability,
    google_secret_manager_secret_iam_member.adminapi-secrets,

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

resource "google_compute_region_network_endpoint_group" "adminapi" {
  count = length(var.adminapi_hosts) > 0 ? 1 : 0

  name     = "adminapi"
  provider = google-beta
  project  = var.project
  region   = var.region

  network_endpoint_type = "SERVERLESS"

  cloud_run {
    service = google_cloud_run_service.adminapi.name
  }
}

resource "google_compute_backend_service" "adminapi" {
  count = length(var.adminapi_hosts) > 0 ? 1 : 0

  provider = google-beta
  name     = "adminapi"
  project  = var.project

  backend {
    group = google_compute_region_network_endpoint_group.adminapi[0].id
  }
  security_policy = google_compute_security_policy.cloud-armor.name
  log_config {
    enable      = var.enable_lb_logging
    sample_rate = var.enable_lb_logging ? 1 : null
  }
}

resource "google_cloud_run_service_iam_member" "adminapi-public" {
  location = google_cloud_run_service.adminapi.location
  project  = google_cloud_run_service.adminapi.project
  service  = google_cloud_run_service.adminapi.name
  role     = "roles/run.invoker"
  member   = "allUsers"
}

output "adminapi_urls" {
  value = concat([google_cloud_run_service.adminapi.status.0.url], formatlist("https://%s", var.adminapi_hosts))
}
