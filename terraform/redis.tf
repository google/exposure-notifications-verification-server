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

locals {
  # redis_secrets is the list of secrets required to connect to and utilize the
  # cache.
  redis_secrets = flatten([
    google_secret_manager_secret.redis-auth.id,
    google_secret_manager_secret.cache-hmac-key.id,
    google_secret_manager_secret.ratelimit-hmac-key.id,
  ])
}

resource "google_redis_instance" "cache" {
  name           = var.redis_name
  tier           = "STANDARD_HA"
  memory_size_gb = var.redis_cache_size

  location_id             = var.redis_location
  alternative_location_id = var.redis_alternative_location

  authorized_network = google_service_networking_connection.private_vpc_connection.network
  connect_mode       = "PRIVATE_SERVICE_ACCESS"

  redis_version = "REDIS_5_0"
  auth_enabled  = var.redis_enable_auth

  depends_on = [
    google_project_service.services["redis.googleapis.com"],
  ]
}

# Create secret for the auth string
resource "google_secret_manager_secret" "redis-auth" {
  secret_id = "redis-auth"

  replication {
    automatic = true
  }

  depends_on = [
    google_project_service.services["secretmanager.googleapis.com"],
  ]
}

resource "google_secret_manager_secret_version" "redis-auth" {
  secret      = google_secret_manager_secret.redis-auth.id
  secret_data = coalesce(google_redis_instance.cache.auth_string, "unused")
}

# Create secret for the HMAC cache keys
resource "random_id" "cache-hmac-key" {
  byte_length = 128
}

resource "google_secret_manager_secret" "cache-hmac-key" {
  secret_id = "cache-hmac-key"

  replication {
    automatic = true
  }

  depends_on = [
    google_project_service.services["secretmanager.googleapis.com"],
  ]
}

resource "google_secret_manager_secret_version" "cache-hmac-key" {
  secret      = google_secret_manager_secret.cache-hmac-key.id
  secret_data = random_id.cache-hmac-key.b64_std
}

# Create secret for the HMAC ratelimit keys
resource "random_id" "ratelimit-hmac-key" {
  byte_length = 128
}

resource "google_secret_manager_secret" "ratelimit-hmac-key" {
  secret_id = "ratelimit-hmac-key"

  replication {
    automatic = true
  }

  depends_on = [
    google_project_service.services["secretmanager.googleapis.com"],
  ]
}

resource "google_secret_manager_secret_version" "ratelimit-hmac-key" {
  secret      = google_secret_manager_secret.ratelimit-hmac-key.id
  secret_data = random_id.ratelimit-hmac-key.b64_std
}

output "redis_host" {
  value = google_redis_instance.cache.host
}

output "redis_port" {
  value = google_redis_instance.cache.port
}
