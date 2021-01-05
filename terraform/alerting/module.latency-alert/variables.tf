variable "custom_service_id" {
  type = string
}

variable "service_name" {
  type = string
}

variable "enabled" {
  type        = bool
  description = "Whether to enable this alert"
}

variable "notification_channels" {
  type        = map(any)
  description = "Notification channels"
}

variable "project" {
  type = string
}

variable "threshold" {
  type        = number
  description = "Latency threshold (in ms)."
}

variable "duration" {
  type        = number
  description = "Duration of alert evaluation"
}

