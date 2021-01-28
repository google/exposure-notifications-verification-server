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
  # database_secrets is the list of secrets required to connect to and utilize
  # the database. It includes the database password as well as access to HMAC
  # keys.
  database_secrets = [
    google_secret_manager_secret.db-secret["sslcert"].id,
    google_secret_manager_secret.db-secret["sslkey"].id,
    google_secret_manager_secret.db-secret["sslrootcert"].id,
    google_secret_manager_secret.db-secret["password"].id,
    google_secret_manager_secret.db-apikey-db-hmac.id,
    google_secret_manager_secret.db-apikey-sig-hmac.id,
    google_secret_manager_secret.db-verification-code-hmac.id,
  ]
}

resource "google_sql_database_instance" "db-inst" {
  project          = var.project
  region           = var.region
  database_version = var.database_version

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

    database_flags {
      name  = "cloudsql.enable_pgaudit"
      value = "on"
    }

    database_flags {
      name  = "pgaudit.log"
      value = "all"
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

  depends_on = [
    google_project_service.services["sqladmin.googleapis.com"],
    google_project_service.services["sql-component.googleapis.com"],
  ]
}

resource "google_sql_database_instance" "replicas" {
  for_each = toset(var.database_failover_replica_regions)

  project          = var.project
  region           = each.key
  database_version = var.database_version

  master_instance_name = google_sql_database_instance.db-inst.name

  // These are REGIONAL replicas, which cannot auto-failover. The default
  // configuration has auto-failover in zones. This is for super disaster
  // recovery in which an entire region is down for an extended period of time.
  replica_configuration {
    failover_target = false
  }

  settings {
    tier              = var.database_tier
    disk_size         = var.database_disk_size_gb
    availability_type = "ZONAL"
    pricing_plan      = "PACKAGE"

    database_flags {
      name  = "autovacuum"
      value = "on"
    }

    database_flags {
      name  = "max_connections"
      value = var.database_max_connections
    }

    ip_configuration {
      require_ssl     = true
      private_network = google_service_networking_connection.private_vpc_connection.network
    }
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
  count       = var.db_apikey_db_hmac_count
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
  secret_data = join(",", reverse(random_id.db-apikey-db-hmac.*.b64_std))
}

# Create secret for signature HMAC for api keys
resource "random_id" "db-apikey-sig-hmac" {
  count       = var.db_apikey_sig_hmac_count
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
  secret_data = join(",", reverse(random_id.db-apikey-sig-hmac.*.b64_std))
}

# Create secret for the database HMAC for verification codes
resource "random_id" "db-verification-code-hmac" {
  count       = var.db_verification_code_hmac_count
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
  secret_data = join(",", reverse(random_id.db-verification-code-hmac.*.b64_std))
}


# Grant Cloud Build the ability to access the database secrets (required to run
# migrations).
resource "google_secret_manager_secret_iam_member" "cloudbuild-db-pwd" {
  secret_id = google_secret_manager_secret.db-secret["password"].id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${local.cloudbuild_email}"

  depends_on = [
    google_project_service.services["cloudbuild.googleapis.com"],
  ]
}

resource "google_secret_manager_secret_iam_member" "cloudbuild-db-apikey-db-hmac" {
  secret_id = google_secret_manager_secret.db-apikey-db-hmac.id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${local.cloudbuild_email}"

  depends_on = [
    google_project_service.services["cloudbuild.googleapis.com"],
  ]
}

resource "google_secret_manager_secret_iam_member" "cloudbuild-db-apikey-sig-hmac" {
  secret_id = google_secret_manager_secret.db-apikey-sig-hmac.id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${local.cloudbuild_email}"

  depends_on = [
    google_project_service.services["cloudbuild.googleapis.com"],
  ]
}

resource "google_secret_manager_secret_iam_member" "cloudbuild-db-verification-code-hmac" {
  secret_id = google_secret_manager_secret.db-verification-code-hmac.id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${local.cloudbuild_email}"

  depends_on = [
    google_project_service.services["cloudbuild.googleapis.com"],
  ]
}

# Grant Cloud Build the ability to connect to Cloud SQL.
resource "google_project_iam_member" "cloudbuild-sql" {
  project = var.project
  role    = "roles/cloudsql.client"
  member  = "serviceAccount:${local.cloudbuild_email}"

  depends_on = [
    google_project_service.services["cloudbuild.googleapis.com"]
  ]
}

# Grant Cloud Build use of the KMS key to run migrations
resource "google_kms_crypto_key_iam_member" "database-database-encrypter" {
  crypto_key_id = google_kms_crypto_key.database-encrypter.self_link
  role          = "roles/cloudkms.cryptoKeyEncrypterDecrypter"
  member        = "serviceAccount:${local.cloudbuild_email}"

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
      LOG_LEVEL                         = "debug"
      TAG                               = "initial"
    }

    command = "${path.module}/../scripts/migrate"
  }

  depends_on = [
    google_sql_database.db,
    google_secret_manager_secret_version.db-apikey-sig-hmac,
    google_project_service.services["cloudbuild.googleapis.com"],
    google_secret_manager_secret_iam_member.cloudbuild-db-pwd,
    google_project_iam_member.cloudbuild-sql,
    null_resource.build,
  ]
}

# Create a storage bucket where database backups will be housed.
resource "google_storage_bucket" "backups" {
  project  = var.project
  name     = "${var.project}-backups"
  location = var.storage_location

  force_destroy               = true
  uniform_bucket_level_access = true

  versioning {
    enabled = true
  }

  lifecycle_rule {
    action {
      type = "Delete"
    }

    condition {
      num_newer_versions = "120" // Default backup is 4x/day * 30 days
    }
  }

  depends_on = [
    google_project_service.services["storage.googleaips.com"],
  ]
}

# Give Cloud SQL the ability to create and manage backups.
resource "google_storage_bucket_iam_member" "instance-objectAdmin" {
  bucket = google_storage_bucket.backups.name
  role   = "roles/storage.objectAdmin"
  member = "serviceAccount:${google_sql_database_instance.db-inst.service_account_email_address}"
}

# Create a service account that can initiate backups.
resource "google_service_account" "database-backuper" {
  project      = data.google_project.project.project_id
  account_id   = "en-database-backuper"
  display_name = "Verification database backuper"
}

# Give the service account the ability to initiate backups - unfortunately this
# has to be granted at the project level and isn't scoped to a single database
# instance.
resource "google_project_iam_member" "database-backuper-cloudsql-viewer" {
  project = var.project
  role    = "roles/cloudsql.viewer"
  member  = "serviceAccount:${google_service_account.database-backuper.email}"
}

# Create a Cloud Scheduler job that generates a backup every interval.
resource "google_cloud_scheduler_job" "backup-database-worker" {
  name             = "backup-database-worker"
  region           = var.cloudscheduler_location
  schedule         = var.database_backup_schedule
  time_zone        = "America/Los_Angeles"
  attempt_deadline = "1800s"

  retry_config {
    retry_count = 1
  }

  http_target {
    http_method = "POST"
    uri         = "${google_sql_database_instance.db-inst.self_link}/export"

    body = base64encode(jsonencode({
      exportContext = {
        fileType  = "SQL"
        uri       = "gs://${google_storage_bucket.backups.name}/database/${google_sql_database.db.name}",
        databases = [google_sql_database.db.name]
        offload   = true,
      }
    }))

    oauth_token {
      service_account_email = google_service_account.database-backuper.email
    }
  }

  depends_on = [
    google_app_engine_application.app,
    google_project_iam_member.database-backuper-cloudsql-viewer,
    google_project_service.services["cloudscheduler.googleapis.com"],
  ]
}


output "db_conn" {
  value = google_sql_database_instance.db-inst.connection_name
}

output "db_host" {
  value = google_sql_database_instance.db-inst.private_ip_address
}

output "db_inst_name" {
  value = google_sql_database_instance.db-inst.name
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

output "migrate_command" {
  value = "PROJECT_ID=\"${var.project}\" DB_CONN=\"${google_sql_database_instance.db-inst.connection_name}\" DB_ENCRYPTION_KEY=\"${google_kms_crypto_key.database-encrypter.self_link}\" DB_APIKEY_DATABASE_KEY=\"secret://${google_secret_manager_secret_version.db-apikey-db-hmac.id}\" DB_APIKEY_SIGNATURE_KEY=\"secret://${google_secret_manager_secret_version.db-apikey-sig-hmac.id}\" DB_VERIFICATION_CODE_DATABASE_KEY=\"secret://${google_secret_manager_secret_version.db-verification-code-hmac.id}\" DB_PASSWORD=\"secret://${google_secret_manager_secret_version.db-secret-version["password"].name}\" DB_NAME=\"${google_sql_database.db.name}\" DB_USER=\"${google_sql_user.user.name}\" DB_DEBUG=\"true\" LOG_LEVEL=\"debug\" ./scripts/migrate"
}

output "proxy_command" {
  value = "cloud_sql_proxy -dir \"$${HOME}/sql\" -instances=${google_sql_database_instance.db-inst.connection_name}=tcp:5432"
}

output "proxy_env" {
  value = "DB_SSLMODE=disable DB_HOST=127.0.0.1 DB_NAME=${google_sql_database.db.name} DB_PORT=5432 DB_USER=${google_sql_user.user.name} DB_PASSWORD=$(gcloud secrets versions access ${google_secret_manager_secret_version.db-secret-version["password"].name})"
}

output "psql_env" {
  value = "PGHOST=127.0.0.1 PGPORT=5432 PGUSER=${google_sql_user.user.name} PGPASSWORD=$(gcloud secrets versions access ${google_secret_manager_secret_version.db-secret-version["password"].name})"
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

output "db_backup_command" {
  value = "gcloud scheduler jobs run backup-database-worker --project ${var.project}"
}
