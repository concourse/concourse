# testflight

![Paper Plane](http://i.imgur.com/C3l6ZI3.jpg)

## about

`testflight` is the integration test suite for Concourse.

## usage

1. Create and upload the releases of `concourse` and `garden-linux` to a local
   bosh-lite director.
2. Deploy it, using `deployment.yml` as a base.
2. Run `ginkgo -r`
