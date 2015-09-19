# concourse [![slack.concourse.ci](http://slack.concourse.ci/badge.svg)](http://slack.concourse.ci)

[![Join the chat at https://gitter.im/concourse/concourse](https://badges.gitter.im/Join%20Chat.svg)](https://gitter.im/concourse/concourse?utm_source=badge&utm_medium=badge&utm_campaign=pr-badge&utm_content=badge)

A [BOSH](https://github.com/cloudfoundry/bosh) release for concourse. The
easiest way to deploy your own instance of concourse on AWS, vSphere,
Openstack or with Vagrant.

* Documentation: [concourse.ci](http://concourse.ci)
* Slack: [concourseci.slack.com](https://concourseci.slack.com) (get an invite at [slack.concourse.ci](http://slack.concourse.ci))
* IRC: [#concourse](http://webchat.freenode.net/?channels=concourse) on Freenode
* Roadmap: [Pivotal Tracker](https://www.pivotaltracker.com/n/projects/1059262)

### Example

Concourse's own CI deployment lives at [ci.concourse.ci][concourse-pipeline].
Its pipeline configurations live in this repo under
[ci/pipelines][concourse-config].

[concourse-pipeline]: https://ci.concourse.ci
[concourse-config]: https://github.com/concourse/concourse/blob/develop/ci/pipelines

### Try it on Vagrant

Pre-built Vagrant boxes are available for VirtualBox and AWS. You can stand up
a new instance pretty quickly without having to clone this repo:

```
vagrant init concourse/lite
vagrant up
```

Browse to [http://192.168.100.4:8080](http://192.168.100.4:8080) and download
the [Fly CLI](http://concourse.ci/fly-cli.html) from the bottom-right.

Follow the [Getting Started](http://concourse.ci/getting-started.html) docs
for more information.
