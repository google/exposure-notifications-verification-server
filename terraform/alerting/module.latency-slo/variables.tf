variable "custom_service_id" {
  type = string
}

variable "service_name" {
  type = string
}

variable "goal" {
  type        = number
  description = "Latency SLO goal."
}

variable "threshold" {
  type        = number
  description = "Latency SLO threshold (in ms)."
}

variable "notification_channels" {
  type        = map(any)
  description = "Notification channels"
}

variable "enable_alert" {
  type        = bool
  description = "Whether to enable the alerts."
}

variable "project" {
  type = string
}
