# Contributing In General
So, you've come here hoping to make Concourse better?  Great!
Concourse welcomes pull-request on any one of the many open-source
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

# Setup

### Tools you will need
- ruby
    - use system provided if you have it
- bosh_cli (ruby-gem and all around bosh goodness)
    - ```gem install bosh_cli bosh_cli_plugin_micro --no-ri --no-rdoc```
- direnv (homebrew installed)
     - ```brew install direnv```
- fly (grab this from Concourse.ci)
    - [fly-binary-darwin](https://ci.concourse.ci/api/v1/cli?arch=amd64&platform=darwin)
    - [fly-binary-linux](https://ci.concourse.ci/api/v1/cli?arch=amd64&platform=linux)
- go (you can install via homebrew)
    - ```brew install go```
- ginkgo (testing framework for go, assuming you grab go first)
    - ```go get github.com/onsi/ginkgo/ginkgo```
- postgresql (you can also install this via homebrew)
    - ```brew install postgresql```
- virtualbox
    - [grab it here](https://www.virtualbox.org/wiki/Downloads)
- vagrant
    - [grab the latest version](https://www.vagrantup.com/downloads.html)

### Setting Up a Bosh-lite
Concourse is a bosh release, so you're probably going to want to setup a 
bosh-litethat you can deploy concourse to before pushing your changes 
to the develop branch.

Jump over to the bosh-lite [repo](https://github.com/cloudfoundry/bosh-lite) 
and follow the instructions provided 
[for virtual-box](https://github.com/cloudfoundry/bosh-lite)
(we recommend virtual-box for the smoothest bootstrapping experience).

### Grabbing the Concourse Release
All set with bosh-lite?  Great!  Let's grab the concourse release 
(just clone the project you are reading this documentation in) and walk 
through a deployment:

- You may notice that Concourse ships with a .envrc file.  We use a tool 
called direnv (mentioned above) to mange your ```$GOPATH```.
- We make extensive use of submodules in this release, you will want to
run ```git submodule update --init``` within your Course clone.

You should now be all set to bosh deploy Concourse.  A bosh-lite 
manifest has been provided for you in the manifests directory.

### Making Changes

Any changes you would like to make should be done at the submodule
level.  This will allow you to run a local testflight easily.

### Your First Testflight

This is where your bosh-lite finally comes in handy.

You're going to deploy your changes to the various submodules
directly to your bosh-lite, this requires a couple of things:

- Commit all of the changes directly to the submodules
(just don't push them)
- Upload a garden-linux-release to your bosh-lite,
you can grab it [here](https://github.com/concourse/concourse/releases)
- cd to the top-level of testflight (it's a submodule)
and run ```./scripts/local-test```
- Sit back and wait for the testflight to pass

### Running ATC Suite

ATC tests are shockingly simple to run (assuming you have
ginkgo / postgresql already installed).

After cloning ATC run:
```ginkgo -p -r```
from the top-level ATC directory

### Fly Testing (What to watch for)

Again, relying on the fact that you have already installed
ginkgo:

After cloning fly run:
```ginkgo -p -r```
within the fly directory you just cloned

### Shipit

Do not attempt to bump any of the submodules while working
within the Concourse release.  You should make
pull-requests to the various submodules from your fork(s) - the
maintainers will figure out how to bump the submodules
appropriately after that and create a new release.
