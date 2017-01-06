# concourse [![slack.concourse.ci](http://slack.concourse.ci/badge.svg)](http://slack.concourse.ci)

[Concourse](https://concourse.ci) is a pipeline-based CI system written in Go.

* Website: [concourse.ci](https://concourse.ci)
* Documentation:
  * [Introduction](https://concourse.ci/introduction.html)
  * [Setting Up](https://concourse.ci/setting-up.html)
  * [Using Concourse](https://concourse.ci/using-concourse.html)
* Slack:
  * [Invitation](http://slack.concourse.ci)
  * [#general](https://concourseci.slack.com) for help and chit-chat
* Roadmap:
  * [GitHub Issues](https://github.com/concourse/concourse/issues)
  * [Pivotal Tracker](https://www.pivotaltracker.com/n/projects/1059262)

## Contributing

Concourse is built on a few components, all written in Go with cutesy
aerospace-themed names. This repository is actually its [BOSH](https://bosh.io)
release, which ties everything together and also serves as the central hub for
GitHub issues.

Each component has its own repository:

* [ATC](https://github.com/concourse/atc) is most of Concourse: it provides
  the API, web UI, and all pipeline orchestration
* [Fly](https://github.com/concourse/fly) is the CLI for interacting with and
  configuring Concourse pipelines
* [TSA](https://github.com/concourse/tsa) is a SSH server used for authorizing
  worker registration
* [Garden](https://github.com/cloudfoundry-incubator/garden) is a generic
  interface for orchestrating containers remotely on a worker
* [Baggageclaim](https://github.com/concourse/baggageclaim) is a server for
  managing caches and artifacts on the workers

To learn more about how they fit together, see [Concourse
Architecture](https://concourse.ci/architecture.html).

### Quick Start

Install GoLang
```
wget https://storage.googleapis.com/golang/go1.7.4.linux-amd64.tar.gz
sudo tar zxvf go1.7.4.linux-amd64.tar.gz -C /usr/local

GOROOT=/usr/local/go
GOPATH=/usr/local
PATH="/usr/local/go/bin:/usr/local/bin:$PATH"
```

Install other dependencies
```
sudo apt-get -y install npm nodejs-legacy make ruby git
sudo npm install -g less elm@0.17.0 uglifyjs less-plugin-clean-css
sudo GOPATH=/usr/local/ /usr/local/go/bin/go get -u github.com/jteeuwen/go-bindata/...
sudo gem install bosh_cli
```

Checkout concourse and fetch git submodules
```
git clone https://github.com/concourse/concourse.git
cd concourse
./scripts/update
(cd src/github.com/concourse/atc && make)
```

Create a release
```
bosh create release --name <release-name> --version <version> --force --with-tarball
```

Target the director
```
# defaults: admin/admin
bosh target 10.0.0.6
```

Upload release
```
bosh upload release --name <release-name> --version <version>
```

Set `release` and `version` in `concourse.yml` then run:
```
bosh deployment concourse.yml
bosh deploy
```
