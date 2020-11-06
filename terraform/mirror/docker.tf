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

resource "google_artifact_registry_repository" "mirrors" {
  provider = google-beta

  project       = var.project
  format        = "DOCKER"
  location      = var.artifact_registry_location
  repository_id = "mirrors"
  description   = "Internally-mirrored images from the public Docker Hub."

  depends_on = [
    google_project_service.services["artifactregistry.googleapis.com"],
  ]
}

resource "google_artifact_registry_repository_iam_member" "mirrors-readers" {
  provider = google-beta

  project    = google_artifact_registry_repository.mirrors.project
  location   = google_artifact_registry_repository.mirrors.location
  repository = google_artifact_registry_repository.mirrors.name

  for_each = toset(var.repository_readers)
  member   = each.key
  role     = "roles/artifactregistry.reader"
}
