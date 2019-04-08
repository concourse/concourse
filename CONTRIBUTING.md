# Contributing to Concourse

It takes a lot of work from a lot of people to build a great CI system. We
really appreciate any and all contributions we receive, and are dedicated to
helping out anyone that wants to be a part of Concourse's development.

This doc will go over the basics of developing Concourse and testing your
changes.

If you run into any trouble, feel free to hang out and ask for help in in
[Discord][discord]! We'll grant you the `@contributors` role on request (just
ask in `#introductions`), which will allow you to chat in the `#contributors`
channel where you can ask for help or get feedback on something you're working
on.


## Contribution process

* [Fork this repo][how-to-fork] into your GitHub account.

* Install the [development dependencies](#development-dependencies) and follow
  the instructions below for [running](#running-concourse) and
  [developing](#developing-concourse) Concourse.

* Commit your changes and push them to a branch on your fork.

  * Don't forget to write tests; pull requests without tests are unlikely to be
    merged. For instruction on writing and running the various test suites, see
    [Testing your changes](#testing-your-changes).

  * All commits must have a signature certifying agreement to the [DCO][dco].
    For more information, see [Signing your work](#signing-your-work).

  * *Optional: check out our [Go style guide][style-guide]!*

* When you're ready, [submit a pull request][how-to-pr]!


## Development dependencies

You'll need a few things installed in order to build, test and run Concourse during
development:

* [`go`](https://golang.org/dl/) v1.11.4+
* [`git`](https://git-scm.com/) v2.11+
* [`yarn`](https://yarnpkg.com/en/docs/install)
* [`docker-compose`](https://docs.docker.com/compose/install/)
* [`postgresql`](https://www.postgresql.org/download/)

> *Concourse uses Go 1.11's module system, so make sure it's **not** cloned
> under your `$GOPATH`.*


## Running Concourse

To build and run Concourse from source, run the following in the root of this
repo:

```sh
$ yarn install
$ yarn build
$ docker-compose up
```

Concourse will be running at [localhost:8080](http://localhost:8080).

### Building `fly` and targeting your local Concourse

To build and install the `fly` CLI from source, run:

```sh
$ go install ./fly
```

This will install a `fly` executable to your `$GOPATH/bin`, so make sure that's
on your `$PATH`!

Once `fly` is built, you can get a test pipeline running like this:

**Log in to the locally-running Concourse instance targeted as `dev`:**
```sh
$ fly -t dev login -c http://localhost:8080 -u test -p test
```

**Create an example pipeline that runs a hello world job every minute:**
```sh
$ fly -t dev set-pipeline -p example -c examples/hello-world-every-minute.yml
```

**Unpause the example pipeline:**
```sh
$ fly -t dev unpause-pipeline -p example
```


## Developing Concourse

Concourse's source code is structured as a monorepo containing Go source code
for the server components and Elm/Less source code for the web UI.

Currently, the top-level folders are ~~confusingly~~ cleverly named, because
they were originally separate components living in their own Git repos with
silly air-traffic-themed names.

| directory       | description |
| :-------------- | :----------- |
| `/atc`          | The "brain" of Concourse: pipeline scheduling, build tracking, resource checking, and web UI/API server. One half of `concourse web`. |
| `/fly`          | The [`fly` CLI](https://concourse-ci.org/fly.html). |
| `/testflight`   | The acceptance test suite, exercising pipeline and `fly` features. Runs against a single Concourse deployment. |
| `/web`          | The Elm source code and other assets for the web UI, which gets built and then embedded into the `concourse` executable and served by the ATC's web server. |
| `/go-concourse` | A Go client libary for using the ATC API, used internally by `fly`. |
| `/skymarshal`   | Adapts [Dex](https://github.com/dexidp/dex) into an embeddable auth component for the ATC, plus the auth flag specifications for `fly` and `concourse web`. |
| `/tsa`          | A custom-built SSH server responsible for securely authenticating and registering workers. The other half of `concourse web`. |
| `/worker`       | The `concourse worker` library code for registering with the TSA, periodically reaping containers/volumes, etc. |
| `/cmd`          | This is mainly glue code to wire the ATC, TSA, [BaggageClaim](https://github.com/concourse/baggageclaim), and Garden into the single `concourse` CLI. |
| `/topgun`       | Another acceptance suite which covers operator-level features and technical aspects of the Concourse runtime. Deploys its own Concourse clusters, runs tests against them, and tears them down. |
| `/ci`           | This folder contains all of our Concourse tasks, pipelines, and Docker images for the Concourse project itself. |

### Rebuilding to test your changes

After making any changes, you can try them out by rebuilding and recreating the
`web` and `worker` containers:

```sh
$ docker-compose up --build -d
```

This can be run in a separate terminal while the original `docker-compose up`
command is still running.

In certain cases, when a change is done to the underlying development image (e.g. Go upgrade from 1.11 to 1.12), you
will need to pull the latest version of `concourse/dev` image, so that `web` and `worker` containers can be built locally
using the fresh image:
```sh
$ docker pull concourse/dev
$ docker-compose up --build -d
```

If you're working on a dependency that doesn't live under this repository (for instance,
`baggageclaim`), you'll need to update `go.mod` with a `replace` directive with the exact
reference that the module lives at:

```sh
# after pushing to the `sample` branch in your baggageclaim fork,
# try to fetch the module revision and get the version.
$ go mod download -json github.com/your-user/baggageclaim@sample | jq '.Version'
go: finding github.com/cirocosta/baggageclaim sample
"v1.3.6-0.20190315100745-09d349f19891"

# with that version, update `go.mod` including a replace directive
$ echo 'replace github.com/concourse/baggageclaim => github.com/your-user/baggageclaim v1.3.6-0.20190315100745-09d349f19891' \
  > ./go.mod

# run the usual build
$ docker-compose up --build -d
```

### Working on the web UI

Concourse is written in Go, but the web UI is written in
[Elm](https://elm-lang.org) and [Less](http://lesscss.org/).

After making changes to `web/`, run the following to rebuild the web UI assets:

```sh
$ yarn build
```

When new assets are built locally, they will automatically propagate to the
`web` container without requiring a restart.

For a quicker feedback cycle, you'll probably want to use `watch` instead of
`build`:

```sh
$ yarn watch
```

This will continuously monitor your local `.elm`/`.less` files and run `yarn
build` whenever they change.

### Debugging with `dlv`

With concourse already running, during local development is possible to attach
[`dlv`](https://github.com/go-delve/delve) to either the `web` or `worker` instance,
allowing you to set breakpoints and inspect the current state of either one of those.

To trace a running web instance:

```sh
$ ./hack/trace web
```

To trace a running worker instance:

```sh
$ ./hack/trace worker
```

### Connecting to Postgres

If you want to poke around the database, you can connect to the `db` node using
the following parameters:

* host: `localhost`
* port: `6543`
* username: `dev`
* password: (blank)
* database: `concourse`

A utility script is provided to connect via `psql` (or `pgcli` if installed):

```sh
$ ./hack/db
```

To reset the database, you'll need to stop everything and then blow away the
`db` container:

```sh
$ docker-compose stop # or Ctrl+C the running session
$ docker-compose rm db
$ docker-compose start
```


## Testing your changes

Any new feature or bug fix should have tests written for it. If there are no
tests, it is unlikely that your pull request will be merged, especially if it's
for a substantial feature.

There are a few different test suites in Concourse:

* **unit tests**: Unit tests live throughout the codebase (`foo_test.go`
  alongside `foo.go`), and should probably be written for any contribution.

* `testflight/`: This suite is the "core Concourse" acceptance tests
  suite, exercising pipeline logic and `fly execute`. A new test should be
  added to `testflight` for most features that are exposed via pipelines or `fly`.

* `web/elm/tests/`: These test the various Elm functions in the web UI code
  in isolation. For the most part, the tests for `web/elm/src/<module name>.elm`
  will be in `web/elm/tests/<module name>Tests.elm`. We have been finding it
  helpful to test the `update` and `view` functions pretty exhaustively,
  leaving the models free to be refactored.

* `web/wats/`: This suite covers specifically the web UI, and run against a
  real Concourse cluster just like `testflight`. This suite is still in its
  early stages and we're working out a unit testing strategy as well, so
  expectations are low for PRs, though we may provide guidance and only require
  coverage on a case-by-case basis.

* `topgun/`: This suite is more heavyweight and exercises behavior that
  may be more visible to operators than end-users. We typically do not expect
  pull requests to add to this suite.

If you need help figuring out the testing strategy for your change, ask in
Discord!

Concourse uses [Ginkgo](http://github.com/onsi/ginkgo) as its test framework
and suite runner of choice for Go code. You'll need to install the `ginkgo` CLI
to run the unit tests and `testflight`:

```sh
$ go get github.com/onsi/ginkgo/ginkgo
```

We use [Counterfeiter](https://github.com/maxbrunsfeld/counterfeiter) to generate
fakes for our unit tests. You may need to regenerate fakes if you add or modify an
interface. To do so, you'll need to install `counterfeiter` as follows:

```sh
$ go get -u https://github.com/maxbrunsfeld/counterfeiter/v6
```

You can then generate the fakes by running

```sh
$ go generate ./...
```

in the directory where the interface is located.

### Running unit tests

Concourse is a ton of code, so it's faster to just run the tests for the
component you're changing.

To run the tests for the package you're in, run:

```sh
$ ginkgo -r -p
```

This will run the tests for all packages found in the current working directory,
recursively (`-r`), running all examples within each package in parallel (`-p`).

You can also pass the path to a package to run as an argument, rather than
`cd`ing.

Note that running `go test ./...` will break, as the tests currently assume only
one package is running at a time (the `ginkgo` default). The `go test` default
is to run each package in parallel, so tests that allocate ports for test
servers and such will collide with each other.

#### Running elm tests

You can run `yarn test` from the root of the repo or `elm-test` from the
`web/elm` directory. They are pretty snappy so you can comfortably run the
whole suite on every change.

### Elm static analysis

Running `yarn analyse` will run many checks across the codebase and report
unused imports and variables, potential optimizations, etc. Powered by
[elm-analyse](https://github.com/stil4m/elm-analyse). If you add the `-s` flag
it will run a server at `localhost:3000` which allows for easier browsing, and
even some automated fixes!

### Elm formatting

Run `yarn format` to format the elm code according to the official Elm Style
Guide. Powered by [elm-format](https://github.com/avh4/elm-format).

### Running the acceptance tests (`testflight`)

The `testflight` package contains tests that run against a real live Concourse.
By default, it will run against `localhost:8080`, i.e. the `docker-compose up`'d
Concourse.

If you've already got Concourse running via `docker-compose up`, you should be
able to just run the acceptance tests by running `ginkgo` the same way you would
run it for unit tests:

```sh
$ ginkgo -r -p testflight
```

Note: because `testflight` actually runs real workloads, you *may* want to limit
the parallelism if you're on a machine with more than, say, 8 cores. This can be
done by specifying `--nodes`:

```sh
$ ginkgo -r --nodes=4 testflight
```

### Running the web acceptance tests (`web/wats`)

Run `yarn test` from the `web/wats` directory. They use puppeteer to run
a headless Chromium. A handy fact is that in most cases if a test fails,
a screenshot taken at the moment of the failure will be at
`web/wats/failure.png`.


### A note on `topgun`

The `topgun/` suite is quite heavyweight and we don't currently expect most
contributors to run or modify it. It's also kind of hard for ~~mere mortals~~
external contributors to run anyway. So for now, ignore it.


## Signing your work

Concourse has joined other open-source projects in adopting the [Developer
Certificate of Origin](https://developercertificate.org/) process for
contributions. The purpose of the DCO is simply to determine that the content
you are contributing is appropriate for submitting under the terms of our
open-source license (Apache v2).

The content of the DCO is as follows:

```
Developer Certificate of Origin
Version 1.1

Copyright (C) 2004, 2006 The Linux Foundation and its contributors.
1 Letterman Drive
Suite D4700
San Francisco, CA, 94129

Everyone is permitted to copy and distribute verbatim copies of this
license document, but changing it is not allowed.


Developer's Certificate of Origin 1.1

By making a contribution to this project, I certify that:

(a) The contribution was created in whole or in part by me and I
    have the right to submit it under the open source license
    indicated in the file; or

(b) The contribution is based upon previous work that, to the best
    of my knowledge, is covered under an appropriate open source
    license and I have the right under that license to submit that
    work with modifications, whether created in whole or in part
    by me, under the same open source license (unless I am
    permitted to submit under a different license), as indicated
    in the file; or

(c) The contribution was provided directly to me by some other
    person who certified (a), (b) or (c) and I have not modified
    it.

(d) I understand and agree that this project and the contribution
    are public and that a record of the contribution (including all
    personal information I submit with it, including my sign-off) is
    maintained indefinitely and may be redistributed consistent with
    this project or the open source license(s) involved.
```

This is also available at <https://developercertificate.org>.

All commits require a `Signed-off-by:` signature indicating that the author has
agreed to the DCO. This must be done using your real name, and must be done on
each commit. This line can be automatically appended via `git commit -s`.

Your commit should look something like this in `git log`:

```
commit 8a0a135f8d3362691235d057896e6fc2a1ca421b (HEAD -> master)
Author: Alex Suraci <asuraci@example.com>
Date:   Tue Dec 18 12:06:07 2018 -0500

    document DCO process

    Signed-off-by: Alex Suraci <asuraci@example.com>
```

If you forgot to add the signature, you can run `git commit --amend -s`. Note
that you will have to force-push (`push -f`) after amending if you've already
pushed commits without the signature.

[discord]: https://discord.gg/MeRxXKW
[dco]: https://developercertificate.org
[style-guide]: https://github.com/concourse/concourse/wiki/Concourse-Go-Style-Guide
[how-to-fork]: https://help.github.com/articles/fork-a-repo/
[how-to-pr]: https://help.github.com/articles/creating-a-pull-request-from-a-fork/
