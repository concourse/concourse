# Contributing

So, you've come here hoping to make Concourse better?  Great!
Concourse welcomes pull-requests on any one of the many open-source
pieces we currently have within our
[organization](https://github.com/concourse).

There are only a few ground rules that we like to see respected for
your pull-request to be considered:

- Make sure that you submit all pull-requests to the develop branch on
Concourse (they won't build otherwise).
- Each corresponding change should have an associated test to go along
with it.  If you are having trouble testing, open an issue to discuss
it.
- Please don't forget to update the Concourse
[documentation](https://github.com/concourse/concourse/tree/develop/docs)
if you make a change to any the behavior (especially in fly).
- Double check that all of the tests you have written pass on the
pull-request

With those ground rules out of the way, let's get you setup to work on
the project!

# Tips & Tricks

Work directly out of the Concourse release `/src` directory!
All of the major Concourse components are there and you can
add your fork of each as a separate downstream remote.  This
will make your time working on Concourse drastically easier.

# ATC-only development

In some cases, you can achieve a more lightweight development cycle by running the ATC outside of a full concourse deployment. For more details, see the [concourse/atc CONTRIBUTING.md](https://github.com/concourse/atc/blob/master/CONTRIBUTING.md). Come back to these instructions to test your changes in a full deployment before submitting a pull request.

# Setup

The tools you will need are highly dependent on the part of Concourse
you are looking to work on, read on for a full list:

### Shared Tooling (the minimum you need to get started)
- ruby (bosh will need this to install the bosh_cli)
    - use system provided if you have it
- bosh_cli (deploy Concourse and manage your bosh-lite)
    - `gem install bosh_cli --no-ri --no-rdoc`
- direnv (used within Concourse to manage your `$GOPATH`)
     - `brew install direnv`
- virtualbox (used by BOSH Lite)
    - `brew install virtualbox`
- vagrant (also used by BOSH Lite)
    - `brew cask install vagrant`

### Additional tooling for ATC or fly
- go (Most of the stack is written in golang)
    - `brew install go`
- phantomjs (used to run our acceptance tests)
    - `brew install phantomjs`
- ginkgo (unit test runner for golang)
    - `go get github.com/onsi/ginkgo/ginkgo`
- postgresql (ATCs database)
    - `brew install postgresql`
- chromedriver (used to run our acceptance tests)
    - `brew install chromedriver`

### Setting Up a BOSH Lite
Concourse is a [BOSH](http://bosh.io/docs)
release, so you're probably going to want to setup a
BOSH Lite that you can deploy concourse to before pushing your changes
to the develop branch.

Jump over to the BOSH Lite [repo](https://github.com/cloudfoundry/bosh-lite)
and follow the instructions provided
[for virtual-box](https://github.com/cloudfoundry/bosh-lite#using-the-virtualbox-provider)
(we recommend virtualbox for the smoothest bootstrapping experience).

Once your BOSH Lite is set up, target it like so:

```
bosh target 192.168.50.4 lite
```

This will point the `bosh` CLI at your local VM and save this target as
the alias `lite`, should you need to target it again.

### Grabbing the Concourse Release
All set with BOSH Lite?  Great!  Let's grab the concourse release
(just clone the project you are reading this documentation in) and walk
through a deployment:

- You may notice that Concourse ships with a .envrc file.  We use a tool
called direnv (mentioned above) to manage your `$GOPATH`.
- We make extensive use of submodules in this release, you will want to
run `git submodule update --init` within your Concourse clone.

You should now be all set to bosh deploy Concourse.  A bosh-lite
manifest has been provided for you in the manifests directory.

### Making Changes

Any changes you would like to make should be done at the submodule
level.  This will allow you to run a local testflight easily.

### Your First Testflight

This is where your BOSH Lite finally comes in handy.

You're going to deploy your changes to the various submodules
directly to your bosh-lite, which requires a couple of things:

- Commit all of the changes directly to the submodules (just don't push them)
- Upload a garden-linux-release to your BOSH Lite, which you can grab from [the Concourse GitHub releases](https://github.com/concourse/concourse/releases)
- Upload the latest BOSH lite stemcell from [bosh.io](http://bosh.io/stemcells/bosh-warden-boshlite-ubuntu-trusty-go_agent)

Then, from the root of the `concourse` repository, run:

```
bosh create release --force
bosh upload release
./src/github.com/concourse/testflight/scripts/local_deploy
```

At this point you should have a Concourse running in your local BOSH Lite
with your changes active. You can browse around it at `http://10.244.15.2:8080`,
and run `testflight`, Concourse's integration suite, by running the following:

```
./src/github.com/concourse/testflight/scripts/test
```

...or by `cd`ing to `src/github.com/concourse/testflight` and running `ginkgo -r`.


### Running ATC Suite

Make sure you've installed all of the related ATC tooling
listed above.  Once that's done, ATC tests are shockingly
simple to run:

After cloning ATC run:
`ginkgo -p -r`
from the top-level ATC directory

#### Building ATC Javascript

```
cd src/github.com/concourse/atc/web
npm install
env PATH=$(npm bin):$PATH make
```

### Fly Testing (What to watch for)

Again, relying on the fact that you have already installed
ginkgo:

After cloning fly run:
`ginkgo -p -r`
within the fly directory you just cloned

### Shipit

Do not attempt to bump any of the submodules while working
within the Concourse release.  You should make
pull-requests to the various submodules from your fork(s) - the
maintainers will figure out how to bump the submodules
appropriately after that and create a new release.
