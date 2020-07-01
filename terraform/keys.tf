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

resource "google_kms_key_ring" "verification" {
  project  = var.project
  name     = "verification"
  location = var.region

  depends_on = [
    google_project_service.services["cloudkms.googleapis.com"],
  ]
}

resource "google_kms_crypto_key" "certificate-signer" {
  key_ring = google_kms_key_ring.verification.self_link
  name     = "certificate-signer"
  purpose  = "ASYMMETRIC_SIGN"

  version_template {
    algorithm        = "EC_SIGN_P256_SHA256"
    protection_level = "HSM"
  }
}

data "google_kms_crypto_key_version" "certificate-signer-version" {
  crypto_key = google_kms_crypto_key.certificate-signer.self_link
}

resource "google_kms_crypto_key" "token-signer" {
  key_ring = google_kms_key_ring.verification.self_link
  name     = "token-signer"
  purpose  = "ASYMMETRIC_SIGN"

  version_template {
    algorithm        = "EC_SIGN_P256_SHA256"
    protection_level = "HSM"
  }
}

data "google_kms_crypto_key_version" "token-signer-version" {
  crypto_key = google_kms_crypto_key.token-signer.self_link
}
