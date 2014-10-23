#lang scribble/manual

@title[#:tag "deploying-with-bosh"]{Deploying a cluster with BOSH}

Once you start needing more workers and @emph{really} caring about your CI
deployment, though, it's best to manage it with BOSH proper.

Using BOSH gives you a self-healing, predictable environment, that can be scaled
up or down by changing a single number.

The @hyperlink["http://docs.cloudfoundry.org/bosh/"]{BOSH documentation} outlines the
concepts, and how to bootstrap on various infrastructures.


@section{Setting up the infrastructure}

Step one is to pick your infrastructure. AWS, vSphere, and OpenStack are fully
supported by BOSH.
@hyperlink["https://github.com/cloudfoundry/bosh-lite"]{BOSH-Lite} is a
pseudo-infrastructure that deploys everything within a single VM. It is a great
way to get started with Concourse and BOSH at the same time, with a faster
feedback loop.

Concourse's infrastructure requirements are fairly straightforward. For example,
Concourse's own pipeline is deployed within an AWS VPC, with its web instances
automatically registered with an Elastic Load Balancer by BOSH.

@hyperlink["http://consul.io"]{Consul} is baked into the BOSH release, so that
you only need to configure static IPs for the jobs running Consul in server
mode, and then configure the Consul agents on the other jobs to join with the
server. This way you can have 100 workers without having to configure them 100
times.


@subsection{BOSH-Lite}

Learning BOSH and your infrastructure at the same time will probably be hard.
If you're getting started with BOSH, you may want to check out
@hyperlink["https://github.com/cloudfoundry/bosh-lite"]{BOSH-Lite} first, which
gives you a fairly BOSHy experience, in a single VM on your machine. This is a
good way to learn the BOSH tooling without having to pay hourly for AWS
instances.


@subsection{AWS}

For AWS, it is recommended to deploy Concourse within a VPC, with the
@code{web} jobs sitting behind an ELB. Registering instances with the ELB is
automated by BOSH; you'll just have to create the ELB itself. This
configuration is more secure, as your CI system's internal jobs aren't exposed
to the outside world; only the webserver.


@subsection{vSphere, OpenStack}

Deploying to vSphere and OpenStack should look roughly the same as the rest, but
this configuration has so far not seen any mileage. You may want to consult the
@hyperlink["http://docs.cloudfoundry.org/bosh/"]{BOSH documentation} instead.


@section{Deploying and upgrading Concourse}

Once you've set up BOSH on your infrastructure, the following steps should get
you started:


@subsection{Upload the stemcell}

A stemcell is the base image for your VMs. It controls the kernel and OS
distribution, and the version of the BOSH agent.

Concourse is tested on the Trusty stemcell. You can find the latest stemcell by
executing @code{bosh public stemcells --full}. Pick the one that matches your
infrastructure, and upload it to your BOSH director with `bosh upload stemcell
<full url>`.


@subsection{Upload the Concourse & Garden releases}

A release is a curated distribution of all of the source bits and configuration
necessary to deploy and scale a product.

A Concourse deployment currently requires two releases:
@hyperlink["http://github.com/concourse/concourse"]{Concourse}'s release
itself, and
@hyperlink["http://github.com/cloudfoundry-incubator/garden-linux-release"]{Garden
Linux}, which it uses for container management.

To upload the releases, simply execute @code{bosh upload release}, with either
a release tarball or a @code{.yml} file from the release repo. Concourse's
release repo includes the correct Garden release as a submodule, guaranteeing
that they're in sync.

To upload the two releases from the Concourse release repo, execute:

@codeblock|{
$ cd concourse/
$ bosh upload release releases/concourse/concourse-X.X.X.yml
$ bosh upload release garden-linux-release/releases/garden-linux/garden-linux-Y.Y.Y.yml
}|

(Replacing the X's and Y's with the highest version number available.)


@subsection{Configure & Deploy}

All you need to deploy your entire Concourse cluster is a BOSH deployment
manifest. This single document describes the desired layout of an entire
cluster.

The Concourse repo contains a few example manifests:

@itemlist[
  @item{
    @hyperlink["https://github.com/concourse/concourse/blob/develop/manifests/bosh-lite.yml"]{BOSH Lite}
  }

  @item{
    @hyperlink["https://github.com/concourse/concourse/blob/develop/manifests/aws-vpc.yml"]{AWS VPC}
  }
]

If you reuse these manifests, you'll probably want to change the following
values:

@itemlist[
  @item{
    @code{director_uuid}: The UUID of your deployment's BOSH director. Obtain
    this with @code{bosh status --uuid}. This is a safeguard against deploying
    to the wrong environments (the risk of making deploys so automated.)
  }

  @item{
    @code{networks}: Your infrastructure's IP ranges and such will probably be
    different, but may end up being the same if you're using AWS with a VPC
    that's the same CIDR block.
  }

  @item{
    @code{jobs.web.networks.X.static_ips} and
    @code{jobs.X.properties.consul.agent.servers.lan}: Pick an internal private
    IP to assign here; this controls how Concourse auto-discovers its internal
    services/workers.
  }

  @item{
    @code{jobs.web.properties.atc.pipeline}: The configuration for your entire
    CI pipeline.
  }

  @item{
    @code{jobs.web.properties.atc.basic_auth_username}
    @code{jobs.web.properties.atc.basic_auth_encrypted_password}: Basic auth
    credentials for the public web server.

    The password must be a BCrypt-encrypted string. Note that higher encryption
    costs may make the site less responsive.
  }

  @item{
    @code{jobs.web.properties.atc.publicly_viewable}: Set to @code{true} to
    make the webserver open (read-only) to the public. Destructive operations
    are not permitted sensitive data is hidden when unauthenticated.
  }

  @item{
    @code{jobs.db.properties.postgresql.roles} and
    @code{jobs.web.properties.atc.postgresql.role}: The credentials to the
    PostgreSQL instance.
  }

  @item{
    @code{jobs.db.persistent_disk}: How much space to give PostgreSQL. You can
    change this at any time; BOSH will safely migrate your persistent data to a
    new disk when scaling up.
  }

  @item{
    @code{jobs.worker.instances}: Change this number to scale up or down the
    number of worker VMs. Concourse will randomly pick a VM out of this pool
    every time it starts a build.
  }

  @item{
    @code{resource_pools}: This is where you configure things like your EC2
    instance type, the ELB to register your instances in, etc.
  }
]

You can change these values at any time and BOSH deploy again, and BOSH will do
The Right Thing™. It will tear down VMs as necessary, but always make sure
persistent data persists, and things come up as they should.

Once you have a deployment manifest, deploying Concourse should simply be:

@codeblock|{
$ bosh deployment path/to/manifest.yml
$ bosh deploy
}|

When new Concourse versions come out, upgrading should simply be a matter of
uploading the new releases and deploying again. BOSH will then kick off a
rolling deploy of your cluster.
