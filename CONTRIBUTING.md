# Improving Concourse

So, you've come here hoping to make Concourse better? Rad. There are lots of
ways to do this, from just hanging around [Slack](http://slack.concourse-ci.org)
to contributing to discussions in GitHub issues and, maybe at some point,
contributing code.

This document will point you in the right direction for whichever ways you
choose to contribute.


# Growing the Community

Contributing to open-source projects can be done in more ways than just
writing code. Often the most valuable thing for a project's growth is to have
constant and healthy *discussions*.

There are a few ways you can help out here:

* If you run into a bug or have a feature request, look for an issue for it, or
  create it if it doesn't already exist. If one already existed, you can either
  add GitHub reactions to it or make a comment providing additional info, but
  please don't leave comments that just say "+1" or "this is affecting me too
  and it's the end of the world".

  GitHub reactions and comments are [literally used to rank the
  issues](https://github.com/vito/customs/blob/master/src/GitHub.elm#L111-L126)
  and will get us to pay more attention to them.

* Just about every large change we're thinking of making will start its life as
  a GitHub issue with the
  [`discuss`](https://github.com/issues?q=is%3Aopen+is%3Aissue+user%3Aconcourse+label%3A%22discuss%22)
  label. These are meant to present a direction we're thinking of going and to
  collect feedback from the community on whether it'd help them.

  The same rules apply for GitHub reactions and comments; they help us decide
  which things are affecting more people.

* Helping other folks out in [Slack](http://slack.concourse-ci.org) and [Stack
  Overflow](http://stackoverflow.com/questions/tagged/concourse) really helps
  grow the community. We try to help out through the day but relying on one
  timezone doesn't scale.



# Contributing Code

If you've got a feature you want to see or a bug you'd like to fix, pull
requests are the way to go.

## Finding things to work on

If you're a generous soul that just wants to give back to the community, but
don't know where to start, check out the [Help Wanted](https://github.com/concourse/concourse/milestone/6)
milestone! We'll typically add things to this milestone that we think are
valuable but don't have the bandwidth to tackle ourselves. They'll usually
be small changes that a contributor could start with.

## Submitting Pull Requests

There are only a few ground rules that we like to see respected for
your pull-request to be considered:

- Each change should have a corresponding test to go along with it. If you are
  having trouble testing, you can just submit the PR ahead of time and we'll be
  happy to help guide you along.

- Pull requests should be focused. Please do not submit changes that mix
  multiple orthogonal changes together, even if you think they're all good.

- All pull requests to the `concourse/concourse` repo should be made to
  `master`. Pull requests to individual components should also be submitted to
  `master`.

- Updating the
  [documentation](https://github.com/concourse/concourse/tree/master/docs) is
  encouraged but not necessary; we'll be sure to cover things before we ship
  the next version if you're not comfortable with writing the docs yourself.

With those ground rules out of the way, let's get you setup to work on
the project!



# Development

## Initial Setup

```
git clone https://github.com/concourse/concourse
cd concourse
git submodule update --init --recursive

# if not using direnv
source .envrc

# if needing to use fly against a local build
cd src/github.com/concourse/fly
go build
# now use the 'fly' in this folder
```

## Running Concourse locally

There are scripts under `dev/` to make running Concourse during development
easier for rapid iteration.

You'll just need the following:

* Go 1.8+
* PostgreSQL
* Docker

To spin up a local Concourse cluster comprising of
[ATC](https://github.com/concourse/atc),
[TSA](https://github.com/concourse/tsa), and a single worker, run:

```sh
./dev/start
```

You can also pass arguments to start only certain components. This can be
useful for starting everything but the bit you're working on, and then starting
that one separately:

```sh
./dev/start db tsa worker # in shell A
./dev/atc                 # in shell B
```

Then you can just `Ctrl+C` the `./dev/atc` process and restart it as you make
changes.

If you prefer to start the postgres database in a container, replace `db` with
`dockerdb` and `atc` with `atc-dockerdb` in `./dev/Procfile`. The `db` Docker
container will persist across `./dev/start` runs. To stop it and perform
cleanup use:

  `docker stop <container ID>`.

### Troubleshooting `dev/worker`
If your `worker` fails to start on macOS with the error
```
docker: Error response from daemon: Mounts denied: EOF
```
a fix is to open your Docker preferences, go to the "File Sharing" tab
and add the directory `/private` to the list
(Click `+` then Cmd + Shift + G to get a "Go to folder" prompt).


## Making changes to Concourse

This repository acts as a Go development workspace, containing all the source
code for Concourse and its dependencies under `src/`. You should first set
`$GOPATH` and `$PATH` appropriately, which can be automated with
[`direnv`](https://direnv.net/).

The other purpose of this repository is to build the [BOSH](https://bosh.io)
release, which is what `jobs/`, `packages/`, and most of the other directories
are for. If you're just contributing to Concourse and don't really care about
BOSH, you can just ignore those. It's purely a convenience for us to have it
all in one place.

Your workflow should consist of `cd`ing to the component you want to change,
checking out the `master` branch (they're submodules, so they default to
pointing to a detached `HEAD`), and working from there, with your `$GOPATH` set
to the root of this repository.

Then, once you're done with your changes, commit locally and push to a branch.
From there you can submit a PR.

Don't worry about bumping the submodules; that tends to be too painful
to synchronize with multiple PRs in flight. We'll take it from there. If your
changes involve multiple components, though (`atc` and `fly` for example), be
sure to let us know in each PR.

When adding / modifying methods to the ATC API, you may need to regenerate
models and data structures used for testing using Counterfeiter. To do so,
download and install counterfeiter:

```bash
go get github.com/maxbrunsfeld/counterfeiter
```

Then, execute the following command from the root of the `go-concourse` repository:

```bash
go generate ./...
```

Don't forget to commit the changes made to this repository when committing your other changes.

# Testing

There are multiple levels of testing in Concourse. If you're adding a feature
or fixing a bug, you should also update the tests. If it's a fairly small
change, it may be enough to just update the component-level tests. If it's
larger though, it may be worth considering adding something to
[Testflight](https://github.com/concourse/testflight). This can also be a nice
place to start as Testflight will definitely show whether or not your shiny new
feature works, and a failing Testflight test is a nice thing to work towards
making green.


## Component-level unit/integration testing

The typical workflow here is: if you're making changes to a single component,
say `atc`, just update the tests and then run:

```sh
./scripts/test
```

This typically just runs `ginkgo -r -p` after doing some additional checks and
balances.

If the component has a `CONTRIBUTING.md` file of its own, be sure to read it -
there may be more to do.


## Integration testing

[Testflight](https://github.com/concourse/testflight) runs against a real live
Concourse and runs `fly` commands and configures pipelines and such. It takes a
little while (on the order of 10 minutes) but is a very good indicator of
whether things actually, like, work.

Running `testflight` should just be a matter of spinning up all the components
using the `dev/` scripts, and running `ginkgo -r` out of the `testflight` repo:

```sh
cd src/github.com/concourse/testflight/
ginkgo -r
```

You may want to speed things up by specifying `-nodes=N` flag. Just don't use
`-p` as things will get a bit slow if there are too many parallel threads
contending for your machine's resources. Good values of `N` are 2 or 3.
