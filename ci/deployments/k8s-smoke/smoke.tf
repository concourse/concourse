variable "project" {
  type = "string"
}

variable "region" {
  type = "string"
  default = "us-central1"
}

provider "google" {
  credentials = "keys/gcp.json"
  project = "${var.project}"
  region = "${var.region}"
}

data "google_compute_zones" "available" {}

resource "google_compute_address" "k8s-smoke" {
  name = "k8s-smoke"
}

resource "random_string" "admin_password" {
  keepers {
    regen = "${uuid()}"
  }

  length = 16
  special = false
}

resource "random_string" "guest_password" {
  keepers {
    regen = "${uuid()}"
  }

  length = 16
  special = false
}

output "instance_ip" {
  value = "${google_compute_address.k8s-smoke.address}"
}

output "admin_password" {
  value = "${random_string.admin_password.result}"
  sensitive = true
}

output "guest_password" {
  value = "${random_string.guest_password.result}"
  sensitive = true
}
