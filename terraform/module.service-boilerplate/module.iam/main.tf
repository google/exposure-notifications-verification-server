resource "google_service_account" "service-account" {
  project      = var.project
  account_id   = "en-verification-${var.service_name}-sa"
  display_name = "Verification Admin apiserver"
}

resource "google_service_account_iam_member" "cloudbuild-deploy" {
  service_account_id = google_service_account.service-account.id
  role               = "roles/iam.serviceAccountUser"
  member             = "serviceAccount:${var.cloud_build_service_account}"
}

resource "google_project_iam_member" "observability" {
  for_each = var.observability_iam_roles
  project  = var.project
  role     = each.key
  member   = "serviceAccount:${google_service_account.service-account.email}"
}

resource "google_kms_key_ring_iam_member" "verification-signer-verifier" {
  key_ring_id = google_kms_key_ring.verification.id
  role        = "roles/cloudkms.signerVerifier"
  member      = "serviceAccount:${google_service_account.service-account.email}"
}

resource "google_kms_crypto_key_iam_member" "database-encrypter" {
  crypto_key_id = google_kms_crypto_key.database-encrypter.id
  role          = "roles/cloudkms.cryptoKeyEncrypterDecrypter"
  member        = "serviceAccount:${google_service_account.service-account.email}"
}
