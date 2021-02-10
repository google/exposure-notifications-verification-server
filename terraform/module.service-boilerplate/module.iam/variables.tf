variable "project" {
  type = string
}

variable "service_name" {
  type = string
}

variable "cloud_build_service_account" {
  type        = string
  description = "Cloud Build service account"
  validation {
    condition     = length(regexall("@cloudbuild.gserviceaccount.com$", var.cloud_build_service_account)) > 0
    error_message = "cloud_build_service_account must ends with @cloudbuild.gserviceaccount.com"
  }
}

variable "observability_iam_roles" {
  type        = set(string)
  description = "Observability IAM roles"

  default = toset([
    "roles/cloudtrace.agent",
    "roles/logging.logWriter",
    "roles/monitoring.metricWriter",
    "roles/stackdriver.resourceMetadata.writer",
  ])
}
