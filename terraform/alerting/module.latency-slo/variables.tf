variable "custom-service-id" {
  type = string
}

variable "service-name" {
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

variable "notification-channels" {
  type        = map(any)
  description = "Notification channels"
}

variable "project" {
  type = string
}
