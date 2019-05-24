variable "project" {
  type = string
}

variable "concourse_tarball" {
  type    = string
  default = "concourse.tgz"
}

variable "region" {
  type    = string
  default = "us-central1"
}

provider "google" {
  credentials = "keys/gcp.json"
  project     = var.project
  region      = var.region
}

data "google_compute_zones" "available" {
}

resource "random_pet" "smoke" {
}

resource "google_compute_address" "smoke" {
  name = "smoke-${random_pet.smoke.id}-ip"
}

resource "google_compute_firewall" "bosh-director" {
  name    = "smoke-${random_pet.smoke.id}-allow-http"
  network = "default"

  allow {
    protocol = "tcp"
    ports    = ["8080"]
  }

  target_tags = ["smoke"]
}

resource "google_compute_instance" "smoke" {
  name         = "smoke-${random_pet.smoke.id}"
  machine_type = "custom-8-8192"
  zone         = data.google_compute_zones.available.names[0]
  tags         = ["smoke"]

  boot_disk {
    initialize_params {
      image = "ubuntu-1804-bionic-v20181003"
      size  = "10"
    }
  }

  network_interface {
    network = "default"

    access_config {
      nat_ip = google_compute_address.smoke.address
    }
  }

  metadata = {
    sshKeys = "root:${file("keys/id_rsa.pub")}"
  }

  connection {
    type        = "ssh"
    host        = google_compute_address.smoke.address
    user        = "root"
    private_key = file("keys/id_rsa")
  }

  provisioner "remote-exec" {
    inline = [
      "set -e -x",
      "apt-get update",
      "apt-get -y install postgresql-10",
      "sudo -i -u postgres createuser concourse",
      "sudo -i -u postgres createdb --owner=concourse concourse",
      "adduser --system --group concourse",
      "mkdir -p /etc/concourse",
      "chgrp concourse /etc/concourse",
    ]
  }
}

resource "random_string" "admin_password" {
  keepers = {
    regen = uuid()
  }

  length  = 16
  special = false
}

resource "random_string" "guest_password" {
  keepers = {
    regen = uuid()
  }

  length  = 16
  special = false
}

data "template_file" "web_conf" {
  template = file("systemd/smoke-web.conf.tpl")

  vars = {
    instance_ip    = google_compute_address.smoke.address
    admin_password = random_string.admin_password.result
    guest_password = random_string.guest_password.result
  }
}

resource "null_resource" "rerun" {
  depends_on = [google_compute_instance.smoke]

  triggers = {
    rerun = uuid()
  }

  connection {
    type        = "ssh"
    host        = google_compute_address.smoke.address
    user        = "root"
    private_key = file("keys/id_rsa")
  }

  provisioner "file" {
    destination = "/tmp/concourse.tgz"
    source      = var.concourse_tarball
  }

  provisioner "remote-exec" {
    inline = [
      "set -e -x",
      "tar -zxf /tmp/concourse.tgz -C /usr/local",
      "mkdir -p /usr/local/concourse/system",
      "mkdir -p /etc/systemd/system/concourse-web.service.d",
      "mkdir -p /etc/systemd/system/concourse-worker.service.d",
    ]
  }

  # TODO: move .service files into tarball and make them official?
  provisioner "file" {
    destination = "/usr/local/concourse/system/concourse-web.service"
    source      = "systemd/concourse-web.service"
  }

  provisioner "file" {
    destination = "/usr/local/concourse/system/concourse-worker.service"
    source      = "systemd/concourse-worker.service"
  }

  provisioner "file" {
    destination = "/etc/systemd/system/concourse-web.service.d/smoke.conf"
    content     = data.template_file.web_conf.rendered
  }

  provisioner "file" {
    destination = "/etc/systemd/system/concourse-worker.service.d/smoke.conf"
    source      = "systemd/smoke-worker.conf"
  }

  provisioner "file" {
    destination = "/etc/concourse/garden.ini"
    source      = "garden/garden.ini"
  }

  provisioner "remote-exec" {
    inline = [
      "set -e -x",
      "export PATH=/usr/local/concourse/bin:$PATH",
      "concourse generate-key -t rsa -f /etc/concourse/session_signing_key",
      "concourse generate-key -t ssh -f /etc/concourse/host_key",
      "concourse generate-key -t ssh -f /etc/concourse/worker_key",
      "cp /etc/concourse/worker_key.pub /etc/concourse/authorized_worker_keys",
      "chgrp concourse /etc/concourse/*",
      "chmod g+r /etc/concourse/*",
      "systemctl enable /usr/local/concourse/system/concourse-web.service",
      "systemctl restart concourse-web.service",
      "systemctl enable /usr/local/concourse/system/concourse-worker.service",
      "systemctl restart concourse-worker.service",
    ]
  }
}

output "instance_ip" {
  value = google_compute_address.smoke.address
}

output "admin_password" {
  value     = random_string.admin_password.result
  sensitive = true
}

output "guest_password" {
  value     = random_string.guest_password.result
  sensitive = true
}
