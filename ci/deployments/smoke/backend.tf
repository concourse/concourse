terraform {
  backend "gcs" {
    credentials = "keys/gcp.json"
    bucket = "concourse-smoke-state"
    prefix = "terraform/5.0-state"
  }
}
