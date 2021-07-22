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

resource "google_firebase_project" "default" {
  provider = google-beta
  project  = var.project

  depends_on = [
    google_project_service.services["firebase.googleapis.com"],
  ]
}

resource "google_firebase_web_app" "default" {
  provider     = google-beta
  project      = google_firebase_project.default.project
  display_name = "Exposure Verifications"
}

data "google_firebase_web_app_config" "default" {
  provider   = google-beta
  web_app_id = google_firebase_web_app.default.app_id
}

output "firebase_app_id" {
  value = google_firebase_web_app.default.app_id
}

output "firebase_project_id" {
  value = google_firebase_web_app.default.project
}

output "firebase_api_key" {
  value = data.google_firebase_web_app_config.default.api_key
}

output "firebase_auth_domain" {
  value = data.google_firebase_web_app_config.default.auth_domain
}

output "firebase_database_url" {
  value = data.google_firebase_web_app_config.default.database_url
}

output "firebase_measurement_id" {
  value = data.google_firebase_web_app_config.default.measurement_id
}

output "firebase_messaging_sender_id" {
  value = data.google_firebase_web_app_config.default.messaging_sender_id
}

output "firebase_storage_bucket" {
  value = data.google_firebase_web_app_config.default.storage_bucket
}
