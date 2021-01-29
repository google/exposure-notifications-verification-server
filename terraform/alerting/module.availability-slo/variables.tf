variable "custom_service_id" {
  type = string
}

variable "service_name" {
  type = string
}

variable "enabled" {
  type        = bool
  description = "Whether to enable this availability SLO."
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

variable "enable_fast_burn_alert" {
  type        = bool
  description = "Whether to enable the fast error budget burn alert."
}

