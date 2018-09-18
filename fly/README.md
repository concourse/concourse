# fly

A command line interface that runs a build in a container with [ATC](https://github.com/concourse/atc).

[The documentation](https://concourse-ci.org/fly.html) is hosted together with Concourse's docs.

A good place to start learning about Concourse is [its documentation](https://concourse-ci.org/index.html)
or its [BOSH release](https://github.com/concourse/concourse).

## Reporting Issues and Requesting Features

Please report all issues and feature requests in [concourse/concourse](https://github.com/concourse/concourse/issues).

## Building

Fly is built using [Go](http://golang.org/). Building and testing fly is most easily done from a checkout of [concourse](https://github.com/concourse/concourse).

1. Check out concourse and update submodules:

  ```bash
  git clone --recursive https://github.com/concourse/concourse.git
  cd concourse
  ```

2. Install [direnv](https://github.com/zimbatm/direnv). Once installed you can `cd` in and out of the concourse
directory to setup your environment.

3. You can now build the fly binary with go build:

  ```bash
  cd src/github.com/concourse/fly
  go build
  ```

4. You can also now run tests by installing and running [ginkgo](http://onsi.github.io/ginkgo/):

  ```bash
  go get github.com/onsi/ginkgo/ginkgo
  ginkgo -r
  ```

## Installing from the Concourse UI for Project Development

Fly is available for download in the lower right-hand corner of the concourse UI.

![fly download links](images/fly_download_ui.png)

1. Navigate to your Concourse instance in the browser, and click the button corresponding to your OS

1. Move the downloaded file onto your PATH

  ```bash
  install ~/Downloads/fly /usr/local/bin
  ```

1. Confirm availability with `which fly`

## Upgrading Fly
Fly is not available for upgrade independently of Concourse. You can download the corresponding upgraded version of Fly via the following: 
* using the [Concourse UI](#installing-from-the-concourse-ui-for-project-development) 
* running `fly -t example sync` if you already have fly locally

