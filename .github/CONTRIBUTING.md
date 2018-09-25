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
* [`yarn`](https://yarnpkg.com/en/docs/install)
* [`docker-compose`](https://docs.docker.com/compose/install/)

You'll also, of course, need to clone this repo:

```sh
$ git clone https://github.com/concourse/concourse
```


## Prerequisite: building the web UI

Concourse is written in Go, but the web UI is written in
[Elm](https://elm-lang.org) and [Less](http://lesscss.org/). Before running
Concourse you'll need to compile them to their `.js`/.`css` assets, like so:

```sh
# install dependencies
$ yarn install

# build Elm/Less source
$ yarn build
```


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

Once `fly` is built, you can target the locally-running Concourse instance like
so:

```sh
$ fly -t dev login -c http://localhost:8080 -u test -p test
```

This will save the target as `dev`, but you can name it whatever you like.

### Rebuilding to test your changes

As you're working on server-side components, you can try out your changes by
rebuilding and recreating the `web` and `worker` containers:

```sh
$ docker-compose up --build -d
```

This can be run while the original `docker-compose up` command is still running.

### Working on the web UI

We already showed how to run `yarn build` during the initial setup, but if
you're actually working on the web UI you'll probably want to use `watch`
instead:

```sh
$ yarn watch
```

This will continuously monitor your local `.elm`/`.less` files and run `yarn
build` whenever they change.

If you're just working on the web UI, you won't need to restart or rebuild the
`docker-compose` containers. The `Dockerfile` mounts the local code to the `web`
container as a shared volume, so changes to the `.js`/`.css` assets will
automatically propagate without needing a restart.


## Connecting to Postgres

If you want to poke around the database, you can connect to the `db` node using
the following parameters:

* host: `localhost`
* port: `6543`
* username: `dev`
* password: (blank)
* database: `concourse`

So you'd connect with something like `psql` like so:

```sh
$ psql -h localhost -p 6543 -U dev concourse
```

To reset the database, you'll need to stop everything and then blow away the
`db` container:

```sh
docker-compose stop # or Ctrl+C the running session
docker-compose rm db
docker-compose start
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
