# example manifests

Here are some example manifests for deploying Concourse in different configurations.

* `bosh-init-worker-vsphere.yml` - to deploy concourse worker on vSphere with bosh-init. We welcome pull requests to keep it up to date.
* `concourse.yml` - concourse BOSH deployment manifest to use with director which is set up with cloud config. Find documentation for setting up cloud config on different infrastructures at [BOSH.io](https://bosh.io/docs/cloud-config.html).
* `bosh-lite.yml` - concourse deployment manifest to use with [BOSH-Lite](https://bosh.io/docs/bosh-lite.html).
