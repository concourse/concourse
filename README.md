# Concourse: the continuous thing-doer.

[![Discord](https://img.shields.io/discord/219899946617274369.svg?label=&logo=discord&logoColor=ffffff&color=7389D8&labelColor=6A7EC2)][discord]
[![Build](https://ci.concourse-ci.org/api/v1/teams/main/pipelines/concourse/badge)](https://ci.concourse-ci.org/teams/main/pipelines/concourse)
[![Contributors](https://img.shields.io/github/contributors/concourse/concourse)](https://github.com/concourse/concourse/graphs/contributors)
[![Help Wanted](https://img.shields.io/github/labels/concourse/concourse/help%20wanted)](https://github.com/concourse/concourse/labels/help%20wanted)

Concourse is an automation system written in Go. Primarily used for CI/CD, it scales to handle automation
pipelines of any complexity.

![booklit pipeline](screenshots/booklit-pipeline.png)

Concourse follows key principles: idempotency, immutability, declarative configuration, stateless workers,
and reproducible builds.

## Installation

Concourse is available as a single `concourse` binary on the [Releases page](https://github.com/concourse/concourse/releases/latest).

If you want to just kick the tires, jump ahead to the [Quick Start](#quick-start).

Other supported distribution formats include:

* [Docker image](https://github.com/concourse/concourse-docker)
* [BOSH release](https://github.com/concourse/concourse-bosh-release)
* [Kubernetes Helm chart](https://github.com/concourse/concourse-chart)

## Quick Start

> [!IMPORTANT]
> Docker Compose provides a simple way to run Concourse on a single node for development, testing,
> or demonstration. While convenient, it's not recommended for production environments.

```sh
$ wget https://concourse-ci.org/docker-compose.yml
$ docker-compose up
Creating docs_concourse-db_1 ... done
Creating docs_concourse_1    ... done
```

Concourse will be running at [127.0.0.1:8080](http://127.0.0.1:8080). Log in with username/password: `test`/`test`.

> [!WARNING]
> **M1 Mac users**: M1 Macs are incompatible with the `containerd` runtime. After downloading the
> docker-compose file, change `CONCOURSE_WORKER_RUNTIME: "containerd"` to `CONCOURSE_WORKER_RUNTIME: "houdini"`.
> **This feature is experimental**

Next, install `fly` by downloading it from the web UI and target your local
Concourse as the `test` user:

```sh
$ fly -t ci login -c http://127.0.0.1:8080 -u test -p test
logging in to team 'main'

target saved
```

### Configuring a Pipeline

Concourse has no GUI for configuration. Instead, pipelines are defined in declarative YAML files:

```yaml
resources:
- name: booklit
  type: git
  source: {uri: "https://github.com/vito/booklit"}

jobs:
- name: unit
  plan:
  - get: booklit
    trigger: true
  - task: test
    file: booklit/ci/test.yml
```

Most operations use the `fly` CLI. With Concourse [installed](https://concourse-ci.org/install.html), save the
example above as `booklit.yml`, [target your Concourse instance](https://concourse-ci.org/fly.html#fly-login), and run:

```sh
fly -t ci set-pipeline -p booklit -c booklit.yml
```

These pipeline files are self-contained, making them easily portable between Concourse instances.

### Learn More

* [Official Site](https://concourse-ci.org) for documentation, reference material, and example pipelines
* [Concourse Tutorial](https://concoursetutorial.com) by Stark & Wayne for a guided introduction to core concepts
* See Concourse in action with our [production pipelines](https://ci.concourse-ci.org/)
* Join our [GitHub discussions](https://github.com/concourse/concourse/discussions) or [Discord](https://discord.gg/MeRxXKW)
* Follow our progress on the [project board](https://github.com/orgs/concourse/projects)

## Contributing

Our user base includes virtually everyone who develops software (and wants it to work).

We welcome your help! If you're interested, check out our [contributing docs](CONTRIBUTING.md).

[discord]: https://discord.gg/MeRxXKW