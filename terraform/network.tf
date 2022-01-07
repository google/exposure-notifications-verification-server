# Copyright 2021 the Exposure Notifications Verification Server authors
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

resource "google_compute_address" "serverless-vpc-addresses" {
  count = var.num_serverless_egress_ips

  project = var.project

  name   = "serverless-vpc-addresses-${count.index}"
  region = var.region

  depends_on = [
    google_project_service.services["compute.googleapis.com"],
  ]
}

resource "google_compute_router" "serverless-vpc-router" {
  project = var.project

  name    = "serverless-vpc-router"
  region  = var.region
  network = "default"

  depends_on = [
    google_project_service.services["compute.googleapis.com"],
  ]
}

resource "google_compute_router_nat" "serverless-vpc-nat" {
  project = var.project

  name   = "serverless-vpc-nat-nat"
  region = var.region
  router = google_compute_router.serverless-vpc-router.name

  nat_ip_allocate_option = "MANUAL_ONLY"
  nat_ips                = google_compute_address.serverless-vpc-addresses.*.self_link

  source_subnetwork_ip_ranges_to_nat = "ALL_SUBNETWORKS_ALL_IP_RANGES"
}

output "serverless_ips" {
  value = google_compute_address.serverless-vpc-addresses.*.address
}
