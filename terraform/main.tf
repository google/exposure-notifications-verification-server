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

provider "google" {
  project = var.project
  region  = var.region

  user_project_override = true
}

provider "google-beta" {
  project = var.project
  region  = var.region

  user_project_override = true
}

provider "random" {}

data "google_project" "project" {
  project_id = var.project
}

resource "google_project_service" "services" {
  project = var.project
  for_each = toset([
    "cloudbuild.googleapis.com",
    "cloudidentity.googleapis.com",
    "cloudkms.googleapis.com",
    "cloudresourcemanager.googleapis.com",
    "cloudscheduler.googleapis.com",
    "compute.googleapis.com",
    "containerregistry.googleapis.com",
    "firebase.googleapis.com",
    "iam.googleapis.com",
    "identitytoolkit.googleapis.com",
    "monitoring.googleapis.com",
    "redis.googleapis.com",
    "run.googleapis.com",
    "secretmanager.googleapis.com",
    "servicenetworking.googleapis.com",
    "sql-component.googleapis.com",
    "sqladmin.googleapis.com",
    "stackdriver.googleapis.com",
    "storage.googleapis.com",
    "vpcaccess.googleapis.com",
  ])
  service            = each.value
  disable_on_destroy = false
}

resource "google_compute_global_address" "private_ip_address" {
  name          = "private-ip-address"
  purpose       = "VPC_PEERING"
  address_type  = "INTERNAL"
  prefix_length = 16
  network       = "projects/${var.project}/global/networks/default"

  depends_on = [
    google_project_service.services["compute.googleapis.com"],
  ]
}

resource "google_service_networking_connection" "private_vpc_connection" {
  network                 = "projects/${var.project}/global/networks/default"
  service                 = "servicenetworking.googleapis.com"
  reserved_peering_ranges = [google_compute_global_address.private_ip_address.name]

  depends_on = [
    google_project_service.services["compute.googleapis.com"],
    google_project_service.services["servicenetworking.googleapis.com"],
  ]
}

resource "google_vpc_access_connector" "connector" {
  project        = var.project
  name           = "serverless-vpc-connector"
  region         = var.region
  network        = "default"
  ip_cidr_range  = "10.8.0.0/28"
  max_throughput = var.vpc_access_connector_max_throughput

  depends_on = [
    google_service_networking_connection.private_vpc_connection,
    google_project_service.services["compute.googleapis.com"],
    google_project_service.services["vpcaccess.googleapis.com"],
  ]
}

resource "null_resource" "build" {
  provisioner "local-exec" {
    environment = {
      PROJECT_ID = var.project
      REGION     = var.region
      SERVICES   = "all"
      TAG        = "initial"
    }

    command = "${path.module}/../scripts/build"
  }

  depends_on = [
    google_project_service.services["cloudbuild.googleapis.com"],
    google_storage_bucket_iam_member.cloudbuild-cache,
  ]
}

resource "google_project_iam_member" "cloudbuild-deploy" {
  project = var.project
  role    = "roles/run.admin"
  member  = "serviceAccount:${data.google_project.project.number}@cloudbuild.gserviceaccount.com"

  depends_on = [
    google_project_service.services["cloudbuild.googleapis.com"],
  ]
}

# Cloud Scheduler requires AppEngine projects!
#
# If your project already has GAE enabled, run `terraform import google_app_engine_application.app $PROJECT_ID`
resource "google_app_engine_application" "app" {
  project     = data.google_project.project.project_id
  location_id = var.appengine_location
}

# Create a helper for generating the local environment configuration - this is
# disabled by default because it includes sensitive information to the project.
resource "local_file" "env" {
  count = var.create_env_file == true ? 1 : 0

  filename        = "${path.root}/.env"
  file_permission = "0600"

  sensitive_content = <<EOF
export PROJECT_ID="${var.project}"

export CSRF_AUTH_KEY="secret://${google_secret_manager_secret_version.csrf-token-version.id}"
export COOKIE_KEYS="secret://${google_secret_manager_secret_version.cookie-hmac-key-version.id},secret://${google_secret_manager_secret_version.cookie-encryption-key-version.id}"

# Note: these configurations assume you're using the Cloud SQL proxy!
export DB_APIKEY_DATABASE_KEY="secret://${google_secret_manager_secret_version.db-apikey-db-hmac.id}"
export DB_APIKEY_SIGNATURE_KEY="secret://${google_secret_manager_secret_version.db-apikey-sig-hmac.id}"
export DB_CONN="${google_sql_database_instance.db-inst.connection_name}"
export DB_DEBUG="true"
export DB_ENCRYPTION_KEY="${google_kms_crypto_key.database-encrypter.self_link}"
export DB_HOST="127.0.0.1"
export DB_NAME="${google_sql_database.db.name}"
export DB_PASSWORD="secret://${google_secret_manager_secret_version.db-secret-version["password"].id}"
export DB_PORT="5432"
export DB_SSLMODE="disable"
export DB_USER="${google_sql_user.user.name}"
export DB_VERIFICATION_CODE_DATABASE_KEY="secret://${google_secret_manager_secret_version.db-verification-code-hmac.id}"

export FIREBASE_API_KEY="${data.google_firebase_web_app_config.default.api_key}"
export FIREBASE_APP_ID="${google_firebase_web_app.default.app_id}"
export FIREBASE_AUTH_DOMAIN="${data.google_firebase_web_app_config.default.auth_domain}"
export FIREBASE_DATABASE_URL="${data.google_firebase_web_app_config.default.database_url}"
export FIREBASE_MEASUREMENT_ID="${data.google_firebase_web_app_config.default.measurement_id}"
export FIREBASE_MESSAGE_SENDER_ID="${data.google_firebase_web_app_config.default.messaging_sender_id}"
export FIREBASE_PROJECT_ID="${google_firebase_web_app.default.project}"
export FIREBASE_STORAGE_BUCKET="${data.google_firebase_web_app_config.default.storage_bucket}"

export CACHE_TYPE="REDIS"
export CACHE_REDIS_HOST="${google_redis_instance.cache.host}"
export CACHE_REDIS_PORT="${google_redis_instance.cache.port}"

export RATE_LIMIT_TYPE="REDIS"
export RATE_LIMIT_TOKENS="60"
export RATE_LIMIT_INTERVAL="1m"
export RATE_LIMIT_REDIS_HOST="${google_redis_instance.cache.host}"
export RATE_LIMIT_REDIS_PORT="${google_redis_instance.cache.port}"

export CERTIFICATE_SIGNING_KEY="${trimprefix(data.google_kms_crypto_key_version.certificate-signer-version.id, "//cloudkms.googleapis.com/v1/")}"
export TOKEN_SIGNING_KEY="${trimprefix(data.google_kms_crypto_key_version.token-signer-version.id, "//cloudkms.googleapis.com/v1/")}"
EOF
}

output "project_id" {
  value = var.project
}

output "project_number" {
  value = data.google_project.project.number
}

output "region" {
  value = var.region
}

output "cloudscheduler_location" {
  value = var.cloudscheduler_location
}

output "next_steps" {
  value = {
    "enable_authentication_providers" = "https://console.firebase.google.com/project/${var.project}/authentication/providers"
  }
}
