# fly

A command line tool for configuration and management of [Concourse](https://github.com/concourse/concourse).

Find comprehensive [documentation](https://concourse-ci.org/fly.html) alongside the Concourse docs.
For those new to Concourse, begin with the [main documentation](https://concourse-ci.org/index.html).

## Reporting Issues and Requesting Features

All issues and feature requests should be submitted to [concourse/concourse](https://github.com/concourse/concourse/issues).

## Building

Fly is developed in [Go](http://golang.org/). For optimal building and testing, work from a
[Concourse](https://github.com/concourse/concourse) repository checkout.

1. Clone the repository and update its submodules:

  ```bash
  git clone --recursive https://github.com/concourse/concourse.git
  cd concourse
  ```

2. Build the fly binary:

  ```bash
  cd fly
  go build
  ```

3. Run tests using [ginkgo](http://onsi.github.io/ginkgo/):

  ```bash
  go get github.com/onsi/ginkgo/v2/ginkgo
  ginkgo -r
  ```

## Installing from the Concourse UI for Project Development

Download fly from the lower right corner of the Concourse UI.

![fly download links](images/fly_download_ui.png)

1. Visit your Concourse instance in a browser and select the button for your operating system

1. Add the downloaded file to your PATH:

  ```bash
  install ~/Downloads/fly /usr/local/bin
  ```

1. Verify installation with `which fly`

## Upgrading Fly
Fly must be upgraded alongside Concourse. Get the matching version by either:
* downloading from the [Concourse UI](#installing-from-the-concourse-ui-for-project-development)
* running `fly -t example sync` if you already have fly installed