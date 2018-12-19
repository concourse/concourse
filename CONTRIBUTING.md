# Contributing to Concourse

It takes a lot of work from a lot of people to build a great CI system. We
really appreciate any and all contributions we receive, and are dedicated to
helping out anyone that wants to be a part of Concourse's development.

This doc will go over the basics of developing Concourse and testing your
changes.

If you run into any trouble, feel free to hang out and ask for help in in
[Discord](https://discord.gg/MeRxXKW)! We'll grant you the `@contributors` role
on request (just ask in `#introductions`), which will allow you to chat in the
`#contributors` channel where you can ask for help or get feedback on something
you're working on.


## Contribution Process

* [Fork this repo](https://help.github.com/articles/fork-a-repo/) into your
  GitHub account.

* Install the [development dependencies](#development-dependencies) and follow
  the instructions below for [building the web
  UI](#prerequisite-building-the-web-ui) and [running
  concourse](#running-concourse)

* Make your changes, then commit and push them to your fork.

  * Don't forget to write tests; pull requests without tests are unlikely to be
    merged. See the [testing section](#testing) below for more details on
    running and writing tests.

  * *Optional: check out our [Go style
    guide](https://github.com/concourse/concourse/wiki/Concourse-Go-Style-Guide)!*

* [Submit a pull
  request!](https://help.github.com/articles/creating-a-pull-request-from-a-fork/)


## Development dependencies

You'll need a few things installed in order to build and run Concourse during
development:

* [`go`](https://golang.org/dl/) v1.11.2+
* [`git`](https://git-scm.com/) v2.11+
* [`yarn`](https://yarnpkg.com/en/docs/install)
* [`docker-compose`](https://docs.docker.com/compose/install/)

> *Concourse uses Go 1.11's module system, so make sure it's **not** cloned
> under your `$GOPATH`.*


## Running Concourse

To build and run a Concourse cluster from source, run the following in the root
of this repo:

```sh
$ docker-compose up
```

Concourse will be running and reachable at
[localhost:8080](http://localhost:8080).


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
| `/bin`          | This is mainly glue code to wire the ATC, TSA, [BaggageClaim](https://github.com/concourse/baggageclaim), and Garden into the single `concourse` CLI. |
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


### Working on the web UI

Concourse is written in Go, but the web UI is written in
[Elm](https://elm-lang.org) and [Less](http://lesscss.org/).

To build the web UI, first install the dependencies:

```sh
$ yarn install
```

Then, run the following to compile everything for the first time:

```sh
$ yarn build
```

These steps are automatically run during `docker-compose up`. When new assets
are built locally, they will automatically propagate to the `web` container
without requiring a restart or rebuild. This works by using a Docker shared
volume and having the dev binary read assets from disk instead of embedding
them.

For a quicker feedback cycle, you'll probably want to use `watch` instead of
`build`:

```sh
$ yarn watch
```

This will continuously monitor your local `.elm`/`.less` files and run `yarn
build` whenever they change.


### Debugging with `dlv`

With concourse already running, during local development is possible to attach
[`dlv`](https://github.com/derekparker/delve) to either the `web` or `worker` instance,
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

A utility script is provided to connect via `psql`:

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


## Testing

Concourse uses [Ginkgo](http://github.com/onsi/ginkgo) as its test framework
and suite runner of choice.

You'll need to install the `ginkgo` CLI to run the tests:

```sh
$ go get github.com/onsi/ginkgo/ginkgo
```

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

Note: because Testflight actually runs real workloads, you *may* want to limit
the parallelism if you're on a machine with more than, say, 8 cores. This can be
done by specifying `--nodes`:

```sh
$ ginkgo -r --nodes=4 testflight
```

### TopGun

The `topgun/` suite is quite heavyweight and we don't currently expect most
contributors to run or modify it. It's also kind of hard for ~~mere mortals~~
external contributors to run anyway. So for now, ignore it.

### Writing tests

Any new feature or bug fix should have tests written for it. If there are no
tests, it is unlikely that your pull request will be merged, especially if it's
for a substantial feature.

Tests should be written using the Ginkgo test framework. A `testflight` test
should be written if you're submitting a new user-facing feature to the "core"
Concourse.

If you need help figuring out the testing strategy for your change, ask in
Discord!
