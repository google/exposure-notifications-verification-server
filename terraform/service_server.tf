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
  member             = "serviceAccount:${local.cloudbuild_email}"
}

resource "google_project_iam_member" "server-observability" {
  for_each = local.observability_iam_roles
  project  = var.project
  role     = each.key
  member   = "serviceAccount:${google_service_account.server.email}"
}

resource "google_project_iam_member" "firebase-admin" {
  project = var.project
  role    = "roles/firebaseauth.admin"
  member  = "serviceAccount:${google_service_account.server.email}"
}

resource "google_kms_key_ring_iam_member" "server-verification-key-admin" {
  key_ring_id = google_kms_key_ring.verification.self_link
  role        = "roles/cloudkms.admin"
  member      = "serviceAccount:${google_service_account.server.email}"
}

resource "google_kms_key_ring_iam_member" "server-verification-key-signer-verifier" {
  key_ring_id = google_kms_key_ring.verification.self_link
  role        = "roles/cloudkms.signerVerifier"
  member      = "serviceAccount:${google_service_account.server.email}"
}

resource "google_kms_crypto_key_iam_member" "server-database-encrypter" {
  crypto_key_id = google_kms_crypto_key.database-encrypter.self_link
  role          = "roles/cloudkms.cryptoKeyEncrypterDecrypter"
  member        = "serviceAccount:${google_service_account.server.email}"
}

locals {
  server_secrets = flatten([
    local.database_secrets,
    local.redis_secrets,
    local.session_secrets,
  ])
}

resource "google_secret_manager_secret_iam_member" "server-secrets" {
  count     = length(local.server_secrets)
  secret_id = element(local.server_secrets, count.index)
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.server.email}"
}

resource "google_cloud_run_service" "server" {
  name     = "server"
  location = var.region

  autogenerate_revision_name = true

  metadata {
    annotations = merge(
      local.default_service_annotations,
      var.default_service_annotations_overrides,
      lookup(var.service_annotations, "server", {})
    )
  }

  template {
    spec {
      service_account_name = google_service_account.server.email
      timeout_seconds      = 60

      containers {
        image = "gcr.io/${var.project}/github.com/google/exposure-notifications-verification-server/server:initial"

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
            local.firebase_config,
            local.gcp_config,
            local.rate_limit_config,
            local.server_config,
            local.session_config,
            local.signing_config,
            local.issue_config,
            local.observability_config,

            // This MUST come last to allow overrides!
            lookup(var.service_environment, "_all", {}),
            lookup(var.service_environment, "server", {}),
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
        lookup(var.revision_annotations, "server", {})
      )
    }
  }

  depends_on = [
    google_project_service.services["run.googleapis.com"],

    google_kms_crypto_key_iam_member.server-database-encrypter,
    google_kms_key_ring_iam_member.server-verification-key-admin,
    google_kms_key_ring_iam_member.server-verification-key-signer-verifier,
    google_project_iam_member.firebase-admin,
    google_project_iam_member.server-observability,
    google_secret_manager_secret_iam_member.server-secrets,


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

resource "google_compute_region_network_endpoint_group" "server" {
  count = length(var.server_hosts) > 0 ? 1 : 0

  name     = "server"
  provider = google-beta
  project  = var.project
  region   = var.region

  network_endpoint_type = "SERVERLESS"

  cloud_run {
    service = google_cloud_run_service.server.name
  }
}

resource "google_compute_backend_service" "server" {
  count = length(var.server_hosts) > 0 ? 1 : 0

  provider = google-beta
  name     = "server"
  project  = var.project

  security_policy = google_compute_security_policy.cloud-armor.name

  backend {
    group = google_compute_region_network_endpoint_group.server[0].id
  }

  log_config {
    enable      = var.enable_lb_logging
    sample_rate = var.enable_lb_logging ? 1 : null
  }
}

resource "google_cloud_run_service_iam_member" "server-public" {
  location = google_cloud_run_service.server.location
  project  = google_cloud_run_service.server.project
  service  = google_cloud_run_service.server.name
  role     = "roles/run.invoker"
  member   = "allUsers"
}

output "server_urls" {
  value = concat([google_cloud_run_service.server.status.0.url], formatlist("https://%s", var.server_hosts))
}
