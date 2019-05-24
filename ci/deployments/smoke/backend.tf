terraform {
  backend "gcs" {
    credentials = "keys/gcp.json"
    bucket      = "concourse-smoke-state"
    prefix      = "terraform/state"
  }
}
