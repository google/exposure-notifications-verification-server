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

resource "random_id" "csrf-token" {
  byte_length = 32
}

resource "google_secret_manager_secret" "csrf-token" {
  provider  = google-beta
  secret_id = "csrf-token"

  replication {
    automatic = true
  }

  depends_on = [
    google_project_service.services["secretmanager.googleapis.com"],
  ]
}

resource "google_secret_manager_secret_version" "csrf-token-version" {
  provider    = google-beta
  secret      = google_secret_manager_secret.csrf-token.id
  secret_data = random_id.csrf-token.b64_std
}
