variable "project" {
  type = "string"
}

variable "concourse_tarball" {
  type = "string"
  default = "concourse.tgz"
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

resource "google_compute_address" "smoke" {
  name = "smoke"
}

resource "google_compute_instance" "smoke" {
  name = "smoke"
  machine_type = "custom-8-8192"
  zone = "${data.google_compute_zones.available.names[0]}"

  boot_disk {
    initialize_params {
      image = "ubuntu-1804-bionic-v20181003"
      size = "10"
    }
  }

  network_interface {
    network = "default"

    access_config {
      nat_ip = "${google_compute_address.smoke.address}"
    }
  }

  metadata {
    sshKeys = "root:${file("keys/id_rsa.pub")}"
  }

  connection {
    type = "ssh"
    user = "root"
    private_key = "${file("keys/id_rsa")}"
  }

  provisioner "remote-exec" {
    inline = [
      "set -e -x",

      "apt-get update",
      "apt-get -y install postgresql",
      "sudo -i -u postgres createuser concourse",
      "sudo -i -u postgres createdb --owner=concourse concourse",

      "mkdir -p /etc/concourse",
      "ssh-keygen -t rsa -f /etc/concourse/host_key -N ''",
      "ssh-keygen -t rsa -f /etc/concourse/session_signing_key -N ''",
      "ssh-keygen -t rsa -f /etc/concourse/worker_key -N ''",
      "cp /etc/concourse/worker_key.pub /etc/concourse/authorized_worker_keys",

      "adduser --system --group concourse",
      "chgrp concourse /etc/concourse/*",
      "chmod g+r /etc/concourse/*",
    ]
  }
}

resource "null_resource" "rerun" {
  depends_on = ["google_compute_instance.smoke"]

  triggers {
    rerun = "${uuid()}"
  }

  connection {
    type = "ssh"
    host = "${google_compute_address.smoke.address}"
    user = "root"
    private_key = "${file("keys/id_rsa")}"
  }

  provisioner "file" {
    destination = "/tmp/concourse.tgz"
    source = "${var.concourse_tarball}"
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
    source = "systemd/concourse-web.service"
  }

  provisioner "file" {
    destination = "/usr/local/concourse/system/concourse-worker.service"
    source = "systemd/concourse-worker.service"
  }

  provisioner "file" {
    destination = "/etc/systemd/system/concourse-web.service.d/smoke.conf"
    source = "systemd/smoke-web.conf"
  }

  provisioner "file" {
    destination = "/etc/systemd/system/concourse-worker.service.d/smoke.conf"
    source = "systemd/smoke-worker.conf"
  }

  provisioner "remote-exec" {
    inline = [
      "set -e -x",

      "systemctl enable /usr/local/concourse/system/concourse-web.service",
      "systemctl restart concourse-web.service",

      "systemctl enable /usr/local/concourse/system/concourse-worker.service",
      "systemctl restart concourse-worker.service",
    ]
  }
}

output "instance_ip" {
  value = "${google_compute_address.smoke.address}"
}
