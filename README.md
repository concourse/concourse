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
* [Guardian](https://github.com/cloudfoundry-incubator/guardian) is a generic
  interface for orchestrating containers remotely on a worker
* [Baggageclaim](https://github.com/concourse/baggageclaim) is a server for
  managing caches and artifacts on the workers

To learn more about how they fit together, see [Concourse
Architecture](https://concourse.ci/architecture.html).
