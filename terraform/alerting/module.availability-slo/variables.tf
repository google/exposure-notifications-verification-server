variable "custom-service-id" {
  type = string
}

variable "service-name" {
  type = string
}

variable "monitoring-host-project" {
  type        = string
  description = <<-EOT
  The host project for multi-project workspace. See also:
  http://cloud/monitoring/workspaces/create#first-multi-project-workspace If
  empty, will use var.verificatin-server-project by default
  EOT
}

variable "enabled" {
  type        = bool
  description = "Whether to enable this availability SLO."
}

variable "goal" {
  type        = number
  description = "Availability SLO goal."
}

variable "notification-channels" {
  type        = map(any)
  description = "Notification channels"
}

variable "verification-server-project" {
  type = string
}
