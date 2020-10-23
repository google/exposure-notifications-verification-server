terraform {
  backend "gcs" {
    bucket = "composite-store-287220-tf-state"
  }
}
