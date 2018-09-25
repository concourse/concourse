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


## Development dependencies

You'll need a few things installed in order to build and run Concourse during
development:

* [`go`](https://golang.org/dl/) v1.11+
* `make`
* [`yarn`](https://yarnpkg.com/en/docs/install)
* [`docker-compose`](https://docs.docker.com/compose/install/)


## Prerequisite: building the web UI

Concourse is written in Go, but the web UI is written in Elm and needs to be
built on its own:

```sh
$ yarn
$ make
```

Everything else will be built in the Docker image.


## Running Concourse

To build and run Concourse from the repo, just run:

```sh
$ docker-compose up
```

### Rebuilding to test your changes

If you're working on server-side components, you can try out your changes by
rebuilding and recreating the `web` and `worker` containers, like so:

```sh
$ docker-compose up --build -d
```

This can be run while the original `docker-compose up` command is still running.

### Working on the web UI

The feedback loop for the web UI is a bit quicker. The `Dockerfile` mounts the
source code over `/src` after building the binary, which means changes to the
frontend assets (compiled Elm code and CSS) will automatically propagate to the
container.

So any time you want to see your changes to the web UI, just run:

```sh
$ make
```

...and then reload your browser.

### Working on `fly`

If you're working on the `fly` CLI, you can install it locally like so:

```sh
$ go install ./fly
```

This will install a `fly` executable to your `$GOPATH/bin`.


## Connecting to Postgres

If you're working on things like the database schema and want to inspect the
database, you can connect to the `db` node using the following parameters:

* host: `localhost`
* port: `6543`
* username: `dev`
* password: (blank)
* database: `concourse`

So you'd connect with something like `psql` like so:

```sh
$ psql -h localhost -p 6543 -U dev concourse
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

### Writing tests

Any new feature or bug fix should have tests written for it. If there are no
tests, it is unlikely that your pull request will be merged, especially if it's
for a substantial feature.

Tests should be written using the Ginkgo test framework. A `testflight` test
should be written if you're submitting a new user-facing feature to the "core"
Concourse.

If you need help figuring out the testing strategy for your change, ask in
Discord!
