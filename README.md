# fly

A command line interface that runs a build in a container with [ATC](https://github.com/concourse/atc).

A good place to start learning about Concourse is its [BOSH release](https://github.com/concourse/concourse).

## Building

Building and testing fly is most easily done from a checkout of [concourse](https://github.com/concourse/concourse).

1. Check out concourse and update submodules:

  ```bash
  git clone git@github.com:concourse/concourse.git
  cd concourse
  git submodule update --init --recursive
  ```

2. Install [direnv](https://github.com/zimbatm/direnv). Once installed you can `cd` in and out of the concourse
directory to setup your environment.

3. You can now build the the fly binary with go build:

  ```bash
  go build
  ```

4. You can also now run tests by installing and running [ginkgo](http://onsi.github.io/ginkgo/):

  ```bash
  go get github.com/onsi/ginkgo/ginkgo
  ginkgo -r
  ```
