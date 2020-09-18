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

locals {
  gcp_config = {
    PROJECT_ID = var.project
  }

  csrf_config = {
    CSRF_AUTH_KEY = "secret://${google_secret_manager_secret_version.csrf-token-version.id}"
  }

  session_config = {
    COOKIE_KEYS = "secret://${google_secret_manager_secret_version.cookie-hmac-key-version.id},secret://${google_secret_manager_secret_version.cookie-encryption-key-version.id}"
  }

  cache_config = {
    CACHE_TYPE       = "REDIS"
    CACHE_HMAC_KEY   = "secret://${google_secret_manager_secret_version.cache-hmac-key.id}"
    CACHE_REDIS_HOST = google_redis_instance.cache.host
    CACHE_REDIS_PORT = google_redis_instance.cache.port
  }

  database_config = {
    DB_KEY_MANAGER                    = "GOOGLE_CLOUD_KMS"
    DB_APIKEY_DATABASE_KEY            = "secret://${google_secret_manager_secret_version.db-apikey-db-hmac.id}"
    DB_APIKEY_SIGNATURE_KEY           = "secret://${google_secret_manager_secret_version.db-apikey-sig-hmac.id}"
    DB_ENCRYPTION_KEY                 = google_kms_crypto_key.database-encrypter.self_link
    DB_HOST                           = google_sql_database_instance.db-inst.private_ip_address
    DB_NAME                           = google_sql_database.db.name
    DB_PASSWORD                       = "secret://${google_secret_manager_secret_version.db-secret-version["password"].id}"
    DB_SSLCERT                        = "secret://${google_secret_manager_secret_version.db-secret-version["sslcert"].id}?target=file"
    DB_SSLKEY                         = "secret://${google_secret_manager_secret_version.db-secret-version["sslkey"].id}?target=file"
    DB_SSLMODE                        = "verify-ca"
    DB_SSLROOTCERT                    = "secret://${google_secret_manager_secret_version.db-secret-version["sslrootcert"].id}?target=file"
    DB_USER                           = google_sql_user.user.name
    DB_VERIFICATION_CODE_DATABASE_KEY = "secret://${google_secret_manager_secret_version.db-verification-code-hmac.id}"
  }

  firebase_config = {
    FIREBASE_API_KEY           = data.google_firebase_web_app_config.default.api_key
    FIREBASE_APP_ID            = google_firebase_web_app.default.app_id
    FIREBASE_AUTH_DOMAIN       = data.google_firebase_web_app_config.default.auth_domain
    FIREBASE_DATABASE_URL      = lookup(data.google_firebase_web_app_config.default, "database_url")
    FIREBASE_MEASUREMENT_ID    = lookup(data.google_firebase_web_app_config.default, "measurement_id")
    FIREBASE_MESSAGE_SENDER_ID = lookup(data.google_firebase_web_app_config.default, "messaging_sender_id")
    FIREBASE_PROJECT_ID        = google_firebase_web_app.default.project
    FIREBASE_STORAGE_BUCKET    = lookup(data.google_firebase_web_app_config.default, "storage_bucket")
  }

  rate_limit_config = {
    RATE_LIMIT_HMAC_KEY   = "secret://${google_secret_manager_secret_version.ratelimit-hmac-key.id}"
    RATE_LIMIT_TYPE       = "REDIS"
    RATE_LIMIT_TOKENS     = "60"
    RATE_LIMIT_INTERVAL   = "1m"
    RATE_LIMIT_REDIS_HOST = google_redis_instance.cache.host
    RATE_LIMIT_REDIS_PORT = google_redis_instance.cache.port
  }

  signing_config = {
    CERTIFICATE_KEY_MANAGER     = "GOOGLE_CLOUD_KMS"
    CERTIFICATE_SIGNING_KEY     = trimprefix(data.google_kms_crypto_key_version.certificate-signer-version.id, "//cloudkms.googleapis.com/v1/")
    CERTIFICATE_SIGNING_KEYRING = google_kms_key_ring.verification.self_link

    TOKEN_KEY_MANAGER = "GOOGLE_CLOUD_KMS"
    TOKEN_SIGNING_KEY = trimprefix(data.google_kms_crypto_key_version.token-signer-version.id, "//cloudkms.googleapis.com/v1/")
  }

  e2e_runner_config = {
    HEALTH_AUTHORITY_CODE   = "com.example"
    KEY_SERVER              = "https://example.com/v1/publish"
    VERIFICATION_ADMIN_API  = google_cloud_run_service.adminapi.status.0.url
    VERIFICATION_SERVER_API = google_cloud_run_service.apiserver.status.0.url
  }

  enx_redirect_config = {
    ASSETS_PATH        = "/assets"
    HOSTNAME_TO_REGION = join(",", [for o in var.redirect_domain_map : format("%s=%s", o.host, o.region)])
  }
}

output "cookie_keys" {
  value = local.session_config["COOKIE_KEYS"]
}
