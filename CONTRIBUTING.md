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


## From ideas to implementation

The Concourse project uses Issues for project backlog and bug reports,
Discussions for support and ideation, and Pull Requests for accepting
contributions.

Bugs can be reported directly as [Issues on the `concourse`
repo][concourse-issues], and questions and technical support can be requested as
[Discussions on the `concourse` repo][concourse-discussions].

The project backlog is maintained by the Concourse team at VMware. Instead of
requesting features, we strongly encourage the use of [Discussions on the `rfcs`
repo][rfcs-discussions] for incubating ideas, with a primary focus on forming a
shared understanding the problem at hand; the context is more important than the
solution itself.

Our collective responsibility as the stewards of the Concourse product is to
uphold its promise of being "automation that scales with your project." This
promise is upheld by keeping the mental overhead small as the automation needs
of your project expand. Thus, requests for core features fall under heavy
scrutiny. Rather than addressing each individual request, we need to think of
the product holistically, steering it carefully and refining or replacing
existing concepts rather than introducing new ones all the time.

While this may feel overkill for requests which seem small in scope, connections
can often be made between features which seem unrelated at the surface. Once
those connections are identified, a higher impact change can often be planned
which satisfies many underlying needs at once while minimizing the amount of
"stuff" that users need to internalize in order to be effective with Concourse.

Once consensus is reached on a direction to take, a proposal can be submitted as
a [Pull Request to the `rfcs` repo][rfcs-prs], following the [RFC
process][rfcs-process], allowing the contributors to collaborate through PR
review. A full blown RFC may not be necessary for smaller changes, but going
through this process can reduce painful feedback cycles for larger code changes.

[Pull Requests to the `concourse` repo][concourse-prs] may be submitted at any
time. The rest of this document provides guidance on how to do just that. In the
end, we are really grateful for any contribution made to the project - whether
that's just chiming in on a discussion, helping others out in the community, or
indeed submitting a code change. Cheers! 🍻

[concourse-issues]: https://github.com/concourse/concourse/issues
[concourse-prs]: https://github.com/concourse/concourse/pulls
[concourse-discussions]: https://github.com/concourse/concourse/discussions
[rfcs-prs]: https://github.com/concourse/rfcs/pulls
[rfcs-discussions]: https://github.com/concourse/rfcs/discussions
[rfcs-process]: https://github.com/concourse/rfcs/blob/master/README.md


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

  * Write release notes by adding onto the `latest.md` file in the
    `release-notes/` directory! For formatting and style examples,
    see previous release notes in the same directory.

  * Run [goimports](https://pkg.go.dev/golang.org/x/tools/cmd/goimports)
    to ensure your code follows our code style guidelines.

  * *Optional: check out our [Go style guide][style-guide]!*

* Putting this all together, here is a sample anatomy of an ideal commit:

    ```
     i       ii         iii
     |       |           |
    web: structure: add var declarations

    Since scripts are run in module mode, they follow the "strict mode"
    semantics. Variables must be declared prior to being assigned (e.g. - iv
    cannot have `x = 1` without declaring x (using var, let, or const)

    concourse/concourse#5131 -------------------------------------------- v

    Signed-off-by: Aidan Oldershaw <aoldershaw@pivotal.io> -------------- vi
    ```

    1. [component changed](#developing-concourse)
    1. [structure vs behaviour](#structure-and-behaviour)
    1. brief, imperative-tense description of the change
    1. a message that [tells a story][fav-commit]
    1. mention the issue this change contributes to solving
    1. [sign-off line](#signing-your-work)

* When you're ready, [submit a pull request][how-to-pr]!

  * You will be invited to join `Contributors` team under `Concourse` org on
    Github. Upon accepting the invite you will be able to login to our CI and
    manage the build of your pull request in
    [PRs pipeline](https://ci.concourse-ci.org/teams/contributor/pipelines/prs).

### Structure and Behaviour

In an ideal world, every pull request is small, but the codebase is large and
sometimes complex changes cannot be avoided. To ease PR reviews, there are a few
practices we've found helpful:

* Focus your commits so that they only change a single component at a time.
* Isolate [structure changes from behaviour changes][sb-changes] and label the
commits appropriately - even better, batch commits of the same type into
contiguous blocks.
* Give clear prose justifications for your changes in the commit messages - it's
not unusual that you do some digging to uncover the motivation for a change,
but if you don't mention it in the commit message the diff can feel pretty
opaque.

## Development dependencies

You'll need a few things installed in order to build, test and run Concourse during
development:

* [`go`](https://golang.org/dl/) v1.13+
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
[concourse/examples](https://github.com/concourse/examples) provides a collection
of example Concourse pipelines. Use its `time-triggered.yml` pipeline to create a
hello world job:

```sh
$ git clone git@github.com:concourse/examples.git
$ fly -t dev set-pipeline -p example -c examples/time-triggered.yml
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

To attach IDE debugger to a running instance, you can use the `--listen` flag followed by a port and the dlv will be started in headless mode listening on the specified port.   

To debug a running web instance:

```sh
$ ./hack/trace web --listen 2345
```

To debug a running worker instance:

```sh
$ ./hack/trace worker --listen 2345
```

After this is done, the final step is to connect your IDE to the debugger with the following parameters:
* host: `localhost`
* port: `2345`

For GoLand you can do so by going to Run | Edit Configurations… | + | Go Remote and fill in the parameters.


### Trying out distributed tracing with Jaeger

Under `./hack`, a docker-compose override file named `jaeger.yml` provides the
essentials to get [Jaeger] running alongside the other components, as well as
tying Concourse to it through the right environment variables.

[Jaeger]: https://jaegertracing.io

To leverage that extension, run `docker-compose up` specifying where all the
yaml files are:

```sh
$ docker-compose \
  -f ./docker-compose.yml \
  -f ./hack/overrides/jaeger.yml \
  up -d
```

### Using the experimental `containerd` garden backend locally

There a docker-compose override (`./hack/overrides/containerd.yml`) that sets up
the necessary environment variables needed to have [`containerd`] up an running as
a Garden backend.

[`containerd`]: https://containerd.io


### Running Vault locally

1. Make sure you have [`certstrap`]
2.  Run `./hack/vault/setup`, and follow the instructions.

See more about in the section [The Vault credential manager].

[The Vault credential Manager]: https://concourse-ci.org/vault-credential-manager.html
[`certstrap`]: https://github.com/square/certstrap


### Running Prometheus locally

Just like for Jaeger, we have a docker-compose override file that enhances the
base `docker-compose.yml` with the [Prometheus] service, bringing with it the
necessary configuraton for collecting Concourse metrics.

[Prometheus]: https://prometheus.io


```sh
$ docker-compose \
  -f ./docker-compose.yml \
  -f ./hack/overrides/prometheus.yml \
  up -d
```

Now head to http://localhost:9090, and you'll be able to graph `concourse_`
Prometheus metrics.


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

### Adding migrations

Concourse database migrations live under `atc/db/migration/migrations`. They are
generated using Concourse's own inbuilt migration library. The migration file
names are of the following format:
```
<migration_version>_<migration_name>.(up|down).(sql|go)
```

The migration version number is the timestamp of the time at which the migration
files are created. This is to ensure that the migrations always run in order.
There is a utility provided to generate migration files, located at
`atc/db/migration/cli`.

To generate a migration, you have two options:

#### The short way

Use the `create-migration` script:

```sh
$ ./atc/scripts/create-migration my_migration_name
# or if you want a go migration,
$ ./atc/scripts/create-migration my_migration_name go
```

#### The long way

1. Build the CLI:
```sh
$ cd atc/db/migration
$ go build -o mig ./cli
```
2. Run the `generate` command. It takes the migration name, file type (SQL or Go)
and optionally, the directory in which to put the migration files (by default,
new migrations are placed in `./migrations`):

```sh
$ ./mig generate -n my_migration_name -t sql
```

This should generate two files for you:
```
1510262030_my_migration_name.down.sql
1510262030_my_migration_name.up.sql
```

Now that the migration files have been created in the right format, you can fill
the database up and down migrations in these files. On startup, `concourse web`
will look for any new migrations in `atc/db/migration/migrations` and will run
them in order.

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
$ go get -u github.com/maxbrunsfeld/counterfeiter/v6
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

### Elm benchmarking

Run `yarn benchmark`.

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
`web/wats/failure.png`. There is a known
[issue](https://github.com/concourse/concourse/issues/3890) with running these
tests in parallel (which is the default setting). The issue stems from an
upstream dependency, so until it is fixed you can run `yarn test --serial` to
avoid it.

### Running Kubernetes tests

Kubernetes-related testing are all end-to-end, living under `topgun/k8s`. They
require access to a real Kubernetes cluster with access granted through a
properly configured `~/.kube/config` file.

[`kind`] is a great choice when it comes to running a local Kubernetes cluster -
all you need is `docker`, and the `kind` CLI. If you wish to run the tests with
a high degree of concurrency, it's advised to have multiple kubernetes nodes.
This can be achieved with the following `kind` config:

```yaml
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
- role: worker
- role: worker
- role: worker
- role: worker
```


With the cluster up, the next step is to have a proper [Tiller] setup (the tests
still run with Helm 2):


```bash
kubectl create serviceaccount \
	--namespace kube-system \
	tiller

kubectl create clusterrolebinding \
	tiller-cluster-rule \
	--clusterrole=cluster-admin \
	--serviceaccount=kube-system:tiller

helm init \
	--service-account tiller \
	--upgrade
```


The tests require a few environment variables to be set:

- `CONCOURSE_IMAGE_TAG` or `CONCOURSE_IMAGE_DIGEST`: the tag or digest to use
when deploying Concourse in the k8s cluster
- `CONCOURSE_IMAGE_NAME`: the name of the image to use when deploying Concourse
to the Kubernetes cluster
- `HELM_CHARTS_DIR`: path to a clone of the [`helm/charts`][helm-charts] repo. This is used
to define the postgres chart that Concourse depends on.
- `CONCOURSE_CHART_DIR`: location in the filesystem where a copy of [`the Concourse Helm
chart`][concourse-helm-chart] exists.


With those set, go to `topgun/k8s` and run Ginkgo:

```sh
# run the test cases serially
ginkgo .

# run the test cases with a concurrency level of 16
ginkgo -nodes=16 .
```

[`kind`]: https://kind.sigs.k8s.io/
[Tiller]: https://v2.helm.sh/docs/install/


### A note on `topgun`

The `topgun/` suite is quite heavyweight and we don't currently expect most
contributors to run or modify it. It's also kind of hard for ~~mere mortals~~
external contributors to run anyway.

To run `topgun`, a BOSH director up and running is required - the only
requirement with regards to where BOSH sits is having the ability to reach the
instance that it creates (the tests make requests to them).

You can have a local setup by leveraging the `dev` scripts in
the [concourse-bosh-release] repo:

[concourse-bosh-release]: https://github.com/concourse/concourse-bosh-release

```bash
# clone the concourse-bosh-release repository
#
git clone https://github.com/concourse/concourse-bosh-release cbr && cd $_


# run the setup script
#
./dev/vbox/setup
```

`setup` will take care of creating the BOSH director (aliased as `vbox`),
uploading basic releases that we need for testing, as well as a BOSH Lite
stemcell.

With the director up, we can head to the tests.

```bash
# fetch the concourse repo, and get into it
#
git clone https://github.com/concourse/concourse concourse && cd $_

# get inside the topgun suite that you want to work on.
#
cd ./topgun/$SUITE

# run the tests of that suite
#
BOSH_ENVIRONMENT=vbox ginkgo -v .
```

ps.: you must have already installed the BOSH cli first.



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
[concourse-helm-chart]: https://github.com/concourse/concourse-chart/blob/master/README.md
[helm-charts]: https://github.com/helm/charts/blob/master/README.md
[fav-commit]: https://dhwthompson.com/2019/my-favourite-git-commit
[sb-changes]: https://medium.com/@kentbeck_7670/bs-changes-e574bc396aaa
