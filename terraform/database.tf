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

resource "random_string" "db-name" {
  length  = 5
  special = false
  number  = false
  upper   = false
}

resource "google_sql_database_instance" "db-inst" {
  project          = var.project
  region           = var.region
  database_version = "POSTGRES_12"
  name             = var.database_name

  settings {
    tier              = var.database_tier
    disk_size         = var.database_disk_size_gb
    availability_type = "REGIONAL"

    database_flags {
      name  = "autovacuum"
      value = "on"
    }

    database_flags {
      name  = "max_connections"
      value = var.database_max_connections
    }

    backup_configuration {
      enabled    = true
      location   = var.database_backup_location
      start_time = "02:00"
    }

    maintenance_window {
      day          = 7
      hour         = 2
      update_track = "stable"
    }

    ip_configuration {
      require_ssl     = true
      private_network = google_service_networking_connection.private_vpc_connection.network
    }
  }

  lifecycle {
    # This prevents accidental deletion of the database.
    prevent_destroy = true
  }

  depends_on = [
    google_project_service.services["sqladmin.googleapis.com"],
    google_project_service.services["sql-component.googleapis.com"],
  ]
}

resource "google_sql_database" "db" {
  project  = var.project
  instance = google_sql_database_instance.db-inst.name
  name     = "verification"
}

resource "google_sql_ssl_cert" "db-cert" {
  project     = var.project
  instance    = google_sql_database_instance.db-inst.name
  common_name = "verification"
}

resource "random_password" "db-password" {
  length  = 64
  special = false
}

resource "google_sql_user" "user" {
  instance = google_sql_database_instance.db-inst.name
  name     = "verification"
  password = random_password.db-password.result
}

resource "google_secret_manager_secret" "db-secret" {
  for_each = toset([
    "sslcert",
    "sslkey",
    "sslrootcert",
    "password",
  ])

  secret_id = "db-${each.key}"

  replication {
    automatic = true
  }

  depends_on = [
    google_project_service.services["secretmanager.googleapis.com"],
  ]
}

resource "google_secret_manager_secret_version" "db-secret-version" {
  for_each = {
    sslcert     = google_sql_ssl_cert.db-cert.cert
    sslkey      = google_sql_ssl_cert.db-cert.private_key
    sslrootcert = google_sql_ssl_cert.db-cert.server_ca_cert
    password    = google_sql_user.user.password
  }

  secret      = google_secret_manager_secret.db-secret[each.key].id
  secret_data = each.value
}

# Create secret for the database HMAC for API keys
resource "random_id" "db-apikey-db-hmac" {
  byte_length = 128
}

resource "google_secret_manager_secret" "db-apikey-db-hmac" {
  secret_id = "db-apikey-db-hmac"

  replication {
    automatic = true
  }

  depends_on = [
    google_project_service.services["secretmanager.googleapis.com"],
  ]
}

resource "google_secret_manager_secret_version" "db-apikey-db-hmac" {
  secret      = google_secret_manager_secret.db-apikey-db-hmac.id
  secret_data = random_id.db-apikey-db-hmac.b64_std
}

# Create secret for signature HMAC for api keys
resource "random_id" "db-apikey-sig-hmac" {
  byte_length = 128
}

resource "google_secret_manager_secret" "db-apikey-sig-hmac" {
  secret_id = "db-apikey-sig-hmac"

  replication {
    automatic = true
  }

  depends_on = [
    google_project_service.services["secretmanager.googleapis.com"],
  ]
}

resource "google_secret_manager_secret_version" "db-apikey-sig-hmac" {
  secret      = google_secret_manager_secret.db-apikey-sig-hmac.id
  secret_data = random_id.db-apikey-sig-hmac.b64_std
}

# Create secret for the database HMAC for verification codes
resource "random_id" "db-verification-code-hmac" {
  byte_length = 128
}

resource "google_secret_manager_secret" "db-verification-code-hmac" {
  secret_id = "db-verification-code-hmac"

  replication {
    automatic = true
  }

  depends_on = [
    google_project_service.services["secretmanager.googleapis.com"],
  ]
}

resource "google_secret_manager_secret_version" "db-verification-code-hmac" {
  secret      = google_secret_manager_secret.db-verification-code-hmac.id
  secret_data = random_id.db-verification-code-hmac.b64_std
}


# Grant Cloud Build the ability to access the database secrets (required to run
# migrations).
resource "google_secret_manager_secret_iam_member" "cloudbuild-db-pwd" {
  secret_id = google_secret_manager_secret.db-secret["password"].id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${data.google_project.project.number}@cloudbuild.gserviceaccount.com"

  depends_on = [
    google_project_service.services["cloudbuild.googleapis.com"],
  ]
}

resource "google_secret_manager_secret_iam_member" "cloudbuild-db-apikey-db-hmac" {
  secret_id = google_secret_manager_secret.db-apikey-db-hmac.id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${data.google_project.project.number}@cloudbuild.gserviceaccount.com"

  depends_on = [
    google_project_service.services["cloudbuild.googleapis.com"],
  ]
}

resource "google_secret_manager_secret_iam_member" "cloudbuild-db-apikey-sig-hmac" {
  secret_id = google_secret_manager_secret.db-apikey-sig-hmac.id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${data.google_project.project.number}@cloudbuild.gserviceaccount.com"

  depends_on = [
    google_project_service.services["cloudbuild.googleapis.com"],
  ]
}

resource "google_secret_manager_secret_iam_member" "cloudbuild-db-verification-code-hmac" {
  secret_id = google_secret_manager_secret.db-verification-code-hmac.id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${data.google_project.project.number}@cloudbuild.gserviceaccount.com"

  depends_on = [
    google_project_service.services["cloudbuild.googleapis.com"],
  ]
}

# Grant Cloud Build the ability to connect to Cloud SQL.
resource "google_project_iam_member" "cloudbuild-sql" {
  project = var.project
  role    = "roles/cloudsql.client"
  member  = "serviceAccount:${data.google_project.project.number}@cloudbuild.gserviceaccount.com"

  depends_on = [
    google_project_service.services["cloudbuild.googleapis.com"]
  ]
}

# Grant Cloud Build use of the KMS key to run migrations
resource "google_kms_crypto_key_iam_member" "database-database-encrypter" {
  crypto_key_id = google_kms_crypto_key.database-encrypter.self_link
  role          = "roles/cloudkms.cryptoKeyEncrypterDecrypter"
  member        = "serviceAccount:${data.google_project.project.number}@cloudbuild.gserviceaccount.com"

  depends_on = [
    google_project_service.services["cloudbuild.googleapis.com"]
  ]
}

# Migrate runs the initial database migrations.
resource "null_resource" "migrate" {
  provisioner "local-exec" {
    environment = {
      PROJECT_ID = var.project
      REGION     = var.region

      DB_APIKEY_DATABASE_KEY            = "secret://${google_secret_manager_secret_version.db-apikey-db-hmac.id}"
      DB_APIKEY_SIGNATURE_KEY           = "secret://${google_secret_manager_secret_version.db-apikey-sig-hmac.id}"
      DB_CONN                           = google_sql_database_instance.db-inst.connection_name
      DB_DEBUG                          = true
      DB_ENCRYPTION_KEY                 = google_kms_crypto_key.database-encrypter.self_link
      DB_NAME                           = google_sql_database.db.name
      DB_PASSWORD                       = "secret://${google_secret_manager_secret_version.db-secret-version["password"].id}"
      DB_USER                           = google_sql_user.user.name
      DB_VERIFICATION_CODE_DATABASE_KEY = "secret://${google_secret_manager_secret_version.db-verification-code-hmac.id}"
      LOG_DEBUG                         = true
    }

    command = "${path.module}/../scripts/migrate"
  }

  depends_on = [
    google_sql_database.db,
    google_secret_manager_secret_version.db-apikey-sig-hmac,
    google_project_service.services["cloudbuild.googleapis.com"],
    google_secret_manager_secret_iam_member.cloudbuild-db-pwd,
    google_project_iam_member.cloudbuild-sql,
  ]
}

output "db_conn" {
  value = google_sql_database_instance.db-inst.connection_name
}

output "db_host" {
  value = google_sql_database_instance.db-inst.private_ip_address
}

output "db_name" {
  value = google_sql_database.db.name
}

output "db_user" {
  value = google_sql_user.user.name
}

output "db_password" {
  value = google_secret_manager_secret_version.db-secret-version["password"].name
}

output "proxy_command" {
  value = "cloud_sql_proxy -dir \"$${HOME}/sql\" -instances=${google_sql_database_instance.db-inst.connection_name}=tcp:5432"
}

output "proxy_env" {
  value = "DB_SSLMODE=disable DB_HOST=127.0.0.1 DB_NAME=${google_sql_database.db.name} DB_PORT=5432 DB_USER=${google_sql_user.user.name} DB_PASSWORD=$(gcloud secrets versions access ${google_secret_manager_secret_version.db-secret-version["password"].name})"
}

output "db_encryption_key_secret" {
  value = google_kms_crypto_key.database-encrypter.self_link
}

output "db_apikey_database_key_secret" {
  value = "secret://${google_secret_manager_secret_version.db-apikey-db-hmac.id}"
}

output "db_apikey_signature_key_secret" {
  value = "secret://${google_secret_manager_secret_version.db-apikey-sig-hmac.id}"
}

output "db_verification_code_key_secret" {
  value = "secret://${google_secret_manager_secret_version.db-verification-code-hmac.id}"
}
