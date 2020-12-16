variable "custom_service_id" {
  type = string
}

variable "service_name" {
  type = string
}

variable "goal" {
  type        = number
  description = "Availability SLO goal."
}

variable "notification_channels" {
  type        = map(any)
  description = "Notification channels"
}

variable "project" {
  type = string
}

variable "enable_alert" {
  type        = bool
  description = "Whether to enable the alerts."
}

