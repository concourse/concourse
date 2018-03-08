# Concourse binary distribution

Builds a single `./concourse` binary capable of running each component of a
Concourse cluster via the following subcommands:

* `web` - runs the [ATC](https://github.com/concourse/atc), web UI and build
  scheduler, alongside a [TSA](https://github.com/concourse/tsa), used to
  securely register workers
* `worker` - runs a [Garden](https://github.com/cloudfoundry/garden) worker and
  registers it via a TSA

## Reporting Issues and Requesting Features

Please report all issues and feature requests in [concourse/concourse](https://github.com/concourse/concourse/issues).

## Usage

See [Standalone Binaries](https://concourse-ci.org/binaries.html).


## Building

Putting these binaries together actually takes quite a bit of heavy lifting.
It's easiest to have them built by Concourse itself using the pipeline and
tasks under `ci/`.

If you're looking to contribute to Concourse, you won't want to start here, as
the feedback loop will be much slower. You should take a look at the
[ATC](https://github.com/concourse/atc) instead.

