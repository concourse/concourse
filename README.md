# concourse

A [BOSH](https://github.com/cloudfoundry/bosh) release for concourse. The
easiest way to deploy your own instance of concourse on AWS, vSphere,
Openstack or with Vagrant.

* Documentation: [concourse.ci](http://concourse.ci)
* IRC: [#concourse](http://webchat.freenode.net/?channels=concourse) on Freenode
* Google Group: [Concourse Users](https://groups.google.com/forum/#!forum/concourse-ci)
* Roadmap: [Pivotal Tracker](https://www.pivotaltracker.com/n/projects/1059262)


### Example

Concourse's own CI deployment lives at
[ci.concourse.ci](https://ci.concourse.ci). Its pipeline configurations live in
this repo under
[ci/pipelines](https://github.com/concourse/concourse/blob/develop/ci/pipelines).


### Try it on Vagrant

Pre-built Vagrant boxes are available for VirtualBox and AWS. You can stand up
a new instance pretty quickly without having to clone this repo:

```
vagrant init concourse/lite
vagrant up
```

Browse to [http://192.168.100.4:8080](http://192.168.100.4:8080) and download
the [Fly CLI](http://concourse.ci/fly-cli.html) from the bottom-right.

Follow the [Provisioning with
Vagrant](http://concourse.ci/deploying-with-vagrant.html) docs for more
information.
