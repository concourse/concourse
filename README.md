# atc ![badge](https://ci.concourse-ci.org/api/v1/teams/main/pipelines/main/jobs/bosh-rc/badge)

*air traffic control - web ui and build scheduler*

![Air Traffic Control](https://farm6.staticflickr.com/5605/15405605898_7ba5062618_d.jpg)

[by](https://creativecommons.org/licenses/by-nc-nd/2.0/) [NATS Press Office](https://www.flickr.com/photos/natspressoffice/)

## reporting issues and requesting features

please report all issues and feature requests in [concourse/concourse](https://github.com/concourse/concourse/issues)

## about

*atc* is the brain of Concourse. It's responsible for scheduling builds across
the cluster of workers, providing the API for the system, as well as serving
the web interface.

It can be scaled horizontally behind a load balancer in order to scale the
system.
