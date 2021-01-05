# Copyright 2021 Google LLC
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
  description = "Duration of alert evaluation (in ms)"
}

