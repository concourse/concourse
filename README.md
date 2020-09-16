# Concourse: the continuous thing-doer.

[![Discord](https://img.shields.io/discord/219899946617274369.svg?label=&logo=discord&logoColor=ffffff&color=7389D8&labelColor=6A7EC2)][discord]
[![Build](https://ci.concourse-ci.org/api/v1/teams/main/pipelines/concourse/badge)](https://ci.concourse-ci.org/teams/main/pipelines/concourse)
[![Contributors](https://img.shields.io/github/contributors/concourse/concourse)](https://github.com/concourse/concourse/graphs/contributors)
[![Help Wanted](https://img.shields.io/github/labels/concourse/concourse/help%20wanted)](https://github.com/concourse/concourse/labels/help%20wanted)

Concourse is an automation system written in Go. It is most commonly used for
CI/CD, and is built to scale to any kind of automation pipeline, from simple to
complex.

![booklit pipeline](screenshots/booklit-pipeline.png)

Concourse is very opinionated about a few things: idempotency, immutability,
declarative config, stateless workers, and reproducible builds.

## The road to Concourse v10

[Concourse v10][v10] is the code name for a set of features which, when used
in combination, will have a massive impact on Concourse's capabilities as a
generic continuous thing-doer. These features, and how they interact, are
described in detail in the [Core roadmap: towards v10][v10] and [Re-inventing
resource types][prototypes] blog posts. (These posts are *slightly* out of
date, but they get the idea across.)

Notably, **v10 will make Concourse not suck for multi-branch and/or
pull-request driven workflows** - examples of *spatial* change, where the set
of things to automate grows and shrinks over time.

Because v10 is really an alias for a ton of separate features, there's a lot
to keep track of - here's an overview:

| Feature                  | RFC              | Status |
| ------------------------ | ---------------- | ------ |
| `set_pipeline` step      | ✔ [#31][rfc-31]  | ✔ v5.8.0 (experimental), TODO: [#5814][issue-5814] |
| Var sources for creds    | ✔ [#39][rfc-39]  | ✔ v5.8.0 (experimental), TODO: [#5813][issue-5813] |
| Archiving pipelines      | ✔ [#33][rfc-33]  | ✔ v6.5.0 |
| Instanced pipelines      | ✔ [#34][rfc-34]  | 🚧 PR [#5896][pr-5896] for backend, issue [#5921][issue-5921] for UI |
| Static `across` step     | 🚧 [#29][rfc-29] | ✔ v6.5.0 (experimental) |
| Dynamic `across` step    | 🚧 [#29][rfc-29] | 🙏 RFC needs feedback! |
| Projects                 | 🚧 [#32][rfc-32] | 🙏 RFC needs feedback! |
| `load_var` step          | ✔ [#27][rfc-27]  | ✔ v6.0.0 (experimental) |
| `get_var` step           | ✔ [#27][rfc-27]  | 🙏 [#5815][issue-5815] Looking for volunteers! |
| [Prototypes][prototypes] | ✔ [#37][rfc-37]  | ⚠ Pending first use of protocol (any of the below) |
| `run` step               | 🚧 [#37][rfc-37]  | ⚠ Pending its own RFC, but feel free to experiment |
| Resource prototypes      | ✔ [#38][rfc-38]  | 🙏 [#5870][issue-5870] Looking for volunteers! |
| Var source prototypes    |                  | ⚠ Needs RFC |
| Notifier prototypes      | 🚧 [#28][rfc-28] | ⚠ RFC not ready |

The Concourse team at VMware will be working on these features, however in the
interest of growing a healthy community of contributors we would really
appreciate any volunteers. This roadmap is very easy to parallelize, as it is
comprised of many orthogonal features, so the faster we can power through it,
the faster we can all benefit. We want these for our own pipelines too! 😆

If you'd like to get involved, hop in [Discord][discord] or leave a comment on
any of the issues linked above so we can coordinate. We're more than happy to
help figure things out or pick up any work that you don't feel comfortable
doing (e.g. UI, unfamiliar parts, etc.).

Thanks to everyone who has contributed so far, whether in code or in the
community, and thanks to everyone for their patience while we figure out how to
support such common functionality the "Concoursey way!" 🙏

[issue-5813]: https://github.com/concourse/concourse/issues/5813
[issue-5814]: https://github.com/concourse/concourse/issues/5814
[issue-5815]: https://github.com/concourse/concourse/issues/5815
[issue-5870]: https://github.com/concourse/concourse/issues/5870
[issue-5921]: https://github.com/concourse/concourse/issues/5921
[pr-5896]: https://github.com/concourse/concourse/pull/5896
[rfc-27]: https://github.com/concourse/rfcs/blob/master/027-var-steps/proposal.md
[rfc-28]: https://github.com/concourse/rfcs/pull/28
[rfc-29]: https://github.com/concourse/rfcs/pull/29
[rfc-31]: https://github.com/concourse/rfcs/blob/master/031-set-pipeline-step/proposal.md
[rfc-32]: https://github.com/concourse/rfcs/pull/32
[rfc-33]: https://github.com/concourse/rfcs/blob/master/033-archiving-pipelines/proposal.md
[rfc-34]: https://github.com/concourse/rfcs/blob/master/034-instanced-pipelines/proposal.md
[rfc-37]: https://github.com/concourse/rfcs/blob/master/037-prototypes/proposal.md
[rfc-38]: https://github.com/concourse/rfcs/blob/master/038-resource-prototypes/proposal.md
[rfc-39]: https://github.com/concourse/rfcs/blob/master/039-var-sources/proposal.md

[v10]: https://blog.concourse-ci.org/core-roadmap-towards-v10/
[prototypes]: https://blog.concourse-ci.org/reinventing-resource-types/

## Installation

Concourse is distributed as a single `concourse` binary, available on the [Releases page](https://github.com/concourse/concourse/releases/latest).

If you want to just kick the tires, jump ahead to the [Quick Start](#quick-start).

In addition to the `concourse` binary, there are a few other supported formats.
Consult their GitHub repos for more information:

* [Docker image](https://github.com/concourse/concourse-docker)
* [BOSH release](https://github.com/concourse/concourse-bosh-release)
* [Kubernetes Helm chart](https://github.com/concourse/concourse-chart)


## Quick Start

```sh
$ wget https://concourse-ci.org/docker-compose.yml
$ docker-compose up
Creating docs_concourse-db_1 ... done
Creating docs_concourse_1    ... done
```

Concourse will be running at [127.0.0.1:8080](http://127.0.0.1:8080). You can
log in with the username/password as `test`/`test`.

Next, install `fly` by downloading it from the web UI and target your local
Concourse as the `test` user:

```sh
$ fly -t ci login -c http://127.0.0.1:8080 -u test -p test
logging in to team 'main'

target saved
```

### Configuring a Pipeline

There is no GUI for configuring Concourse. Instead, pipelines are configured as
declarative YAML files:

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

Most operations are done via the accompanying `fly` CLI. If you've got Concourse
[installed](https://concourse-ci.org/install.html), try saving the above example
as `booklit.yml`, [target your Concourse
instance](https://concourse-ci.org/fly.html#fly-login), and then run:

```sh
fly -t ci set-pipeline -p booklit -c booklit.yml
```

These pipeline files are self-contained, maximizing portability from one
Concourse instance to the next.


### Learn More

* The [Official Site](https://concourse-ci.org) for documentation,
  reference material, and example pipelines (which no longer live in this repository).
* The [Concourse Tutorial](https://concoursetutorial.com) by Stark & Wayne is
  great for a guided introduction to all the core concepts.
* See Concourse in action with our [production pipelines](https://ci.concourse-ci.org/)
* Hang around in the [forums](https://discuss.concourse-ci.org) or in
  [Discord](https://discord.gg/MeRxXKW).
* See what we're working on on the [project board](https://github.com/orgs/concourse/projects). 


## Contributing

Our user base is basically everyone that develops software (and wants it to
work).

It's a lot of work, and we need your help! If you're interested, check out our
[contributing docs](CONTRIBUTING.md).

[discord]: https://discord.gg/MeRxXKW
