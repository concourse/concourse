# Cessna

## Running Tests

- Clone the [echo-resource](github.com/concourse/echo-resource) and follow its instructions to turn it into a RootFS tar
- Ensure you have a running concourse (e.g. on bosh-lite) and take note of the IP address of a worker
- Run

      env WORKER_IP=<worker-ip> TAR_PATH=<path/to/rootfs.tar> ginkgo -resource -p cessna