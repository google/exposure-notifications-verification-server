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

resource "google_service_account" "firebase" {
  project      = var.project
  account_id   = "firebase"
  display_name = "Firebase automation"
}

resource "google_service_account_key" "firebase" {
  service_account_id = google_service_account.firebase.name
}

resource "google_project_iam_member" "firebase" {
  for_each = toset([
    "roles/firebase.admin",
    "roles/serviceusage.serviceUsageAdmin",
  ])

  project = var.project
  role    = each.value
  member  = "serviceAccount:${google_service_account.firebase.email}"
}

provider "google-beta" {
  alias       = "firebase"
  project     = var.project
  region      = var.region
  credentials = base64decode(google_service_account_key.firebase.private_key)
}

resource "google_firebase_project" "default" {
  provider = google-beta.firebase
  project  = var.project

  depends_on = [
    google_project_iam_member.firebase,
    google_project_service.services["firebase.googleapis.com"],
  ]
}

resource "google_firebase_web_app" "default" {
  provider     = google-beta.firebase
  project      = google_firebase_project.default.project
  display_name = "Exposure Verifications"
}

data "google_firebase_web_app_config" "default" {
  provider   = google-beta.firebase
  web_app_id = google_firebase_web_app.default.app_id
}
