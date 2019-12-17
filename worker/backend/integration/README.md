## Running integration tests

These integration tests require a real containerd daemon and runc binary to be present on a host.

#### In Docker

From the project root, build the Dockerfile:

```bash
docker build -t concourse/containerd-test .
```

Run the container in privileged mode, with volume mounting for faster feedback loops:

```bash
docker run --privileged -v CONCOURSE_DIR:/go/src/github.com/concourse/concourse -it concourse/containerd-test /bin/bash
```

Optionally mount the entire $GOPATH to avoid re-pulling go modules:

```bash
docker run --privileged -v $PWD:/go/src/github.com/concourse/concourse -v $GOPATH:/go -it concourse/containerd-test /bin/bash
```

Hit 'enter' once when you're in the container to get back into the bash shell. Navigate to `/go/src/github.com/concourse/concourse`
and run integration tests in this directory with `go test -v`.
