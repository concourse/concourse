# concourse 

[Concourse](https://concourse-ci.org) is a pipeline-based CI system written in Go.

* Website: [concourse-ci.org](https://concourse-ci.org)
* Documentation:
  * [About](https://concourse-ci.org/about.html)
  * [Setting Up](https://concourse-ci.org/install.html)
  * [Using Concourse / Docs](https://concourse-ci.org/docs.html)
* Forum 
  * [Concourse Discourse](https://discuss.concourse-ci.org)
* Chat
  * [Discord](https://discord.gg/MeRxXKW)
* Roadmap:
  * [GitHub Milestones](https://github.com/concourse/concourse/milestones)
  * [GitHub Issues](https://github.com/concourse/concourse/issues)

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

You can get Concourse up and running on your local machine by following the Contributors guide in [`CONTRIBUTING.md`](https://github.com/concourse/concourse/blob/master/CONTRIBUTING.md)

To learn more about how they fit together, see [Concourse
Architecture](https://concourse-ci.org/architecture.html).

Please note that Concourse is released with a Contributor Code of Conduct.
By participating in this project you agree to abide by its terms. You can review
the Code of Code of Conduct in [`CODE_OF_CONDUCT.md`](https://github.com/concourse/concourse/blob/master/CODE_OF_CONDUCT.md) 
