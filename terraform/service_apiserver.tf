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

resource "google_service_account" "apiserver" {
  project      = var.project
  account_id   = "en-verification-apiserver-sa"
  display_name = "Verification apiserver"
}

resource "google_service_account_iam_member" "cloudbuild-deploy-apiserver" {
  service_account_id = google_service_account.apiserver.id
  role               = "roles/iam.serviceAccountUser"
  member             = "serviceAccount:${local.cloudbuild_email}"
}

resource "google_project_iam_member" "apiserver-observability" {
  for_each = local.observability_iam_roles
  project  = var.project
  role     = each.key
  member   = "serviceAccount:${google_service_account.apiserver.email}"
}

resource "google_kms_key_ring_iam_member" "apiserver-verification-signer-verifier" {
  key_ring_id = google_kms_key_ring.verification.self_link
  role        = "roles/cloudkms.signerVerifier"
  member      = "serviceAccount:${google_service_account.apiserver.email}"
}

resource "google_kms_crypto_key_iam_member" "apiserver-database-encrypter" {
  crypto_key_id = google_kms_crypto_key.database-encrypter.self_link
  role          = "roles/cloudkms.cryptoKeyEncrypterDecrypter"
  member        = "serviceAccount:${google_service_account.apiserver.email}"
}

locals {
  apiserver_secrets = flatten([
    local.database_secrets,
    local.redis_secrets,
  ])
}

resource "google_secret_manager_secret_iam_member" "apiserver-secrets" {
  count     = length(local.apiserver_secrets)
  secret_id = element(local.apiserver_secrets, count.index)
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.apiserver.email}"
}

resource "google_cloud_run_service" "apiserver" {
  name     = "apiserver"
  location = var.region

  autogenerate_revision_name = true

  metadata {
    annotations = merge(
      local.default_service_annotations,
      var.default_service_annotations_overrides,
      lookup(var.service_annotations, "apiserver", {})
    )
  }

  template {
    spec {
      service_account_name = google_service_account.apiserver.email
      timeout_seconds      = 25

      containers {
        image = "gcr.io/${var.project}/github.com/google/exposure-notifications-verification-server/apiserver:initial"

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
            local.database_config,
            local.firebase_config,
            local.gcp_config,
            local.rate_limit_config,
            local.signing_config,
            local.observability_config,

            // This MUST come last to allow overrides!
            lookup(var.service_environment, "apiserver", {}),
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
        lookup(var.revision_annotations, "apiserver", {})
      )
    }
  }

  depends_on = [
    google_project_service.services["run.googleapis.com"],

    google_kms_crypto_key_iam_member.apiserver-database-encrypter,
    google_kms_key_ring_iam_member.apiserver-verification-signer-verifier,
    google_project_iam_member.apiserver-observability,
    google_secret_manager_secret_iam_member.apiserver-secrets,

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

resource "google_compute_region_network_endpoint_group" "apiserver" {
  count = length(var.apiserver_hosts) > 0 ? 1 : 0

  name     = "apiserver"
  provider = google-beta
  project  = var.project
  region   = var.region

  network_endpoint_type = "SERVERLESS"

  cloud_run {
    service = google_cloud_run_service.apiserver.name
  }
}

resource "google_compute_backend_service" "apiserver" {
  count = length(var.apiserver_hosts) > 0 ? 1 : 0

  provider = google-beta
  name     = "apiserver"
  project  = var.project

  backend {
    group = google_compute_region_network_endpoint_group.apiserver[0].id
  }
  security_policy = google_compute_security_policy.cloud-armor.name
  log_config {
    enable      = var.enable_lb_logging
    sample_rate = var.enable_lb_logging ? 1 : null
  }
}

resource "google_cloud_run_service_iam_member" "apiserver-public" {
  location = google_cloud_run_service.apiserver.location
  project  = google_cloud_run_service.apiserver.project
  service  = google_cloud_run_service.apiserver.name
  role     = "roles/run.invoker"
  member   = "allUsers"
}

output "apiserver_urls" {
  value = concat([google_cloud_run_service.apiserver.status.0.url], formatlist("https://%s", var.apiserver_hosts))
}
