#lang scribble/manual

@(require "../common.rkt")

@title[#:version version #:tag "worker-pools"]{Worker Pools}

Workers are @hyperlink["http://github.com/cloudfoundry-incubator/garden"]{Garden}
servers, continuously heartbeating their presence to the Concourse API. Workers
have a statically configured @code{platform} and a set of @code{tags}, both of
which determine where tasks are scheduled.

The worker also advertises which resource types it supports. This is just a
mapping from resource type (e.g. @code{git}) to the image URI
(e.g. @code{/var/vcap/packages/...} or @code{docker:///concourse/git-resource}).

For a given resource, the workers declaring that they support its type are used
to run its containers. The @code{platform} does not matter in this case.

For a given task, only workers with a @code{platform} that matches the
task's @code{platform} will be chosen. A worker's @code{platform} is
typically one of @code{linux}, @code{windows}, or @code{darwin}, but this is
just convention.

If a worker specifies @code{tags}, it is taken out of the "default" placement
pool, and tasks only run on the worker if they explicitly specify a common
subset of the worker's tags. This can be done in the task's configuration
directly (see @secref{configuring-tasks}) or by specifying them in the job
that's running the task (see @secref{pipelines}).

Worker registration is done via the ATC API. You can see the current set of
workers via @code{GET /api/v1/workers}, and register one via @code{POST
/api/v1/workers} with a worker API object as the payload.

For example:

@codeblock|{
  {
    "platform": "linux",
    "tags": ["hetzner"],
    "addr": "10.0.16.10:8080",
    "active_containers": 123,
    "resource_types": [
      {"type": "git", "image": "/var/vcap/packages/git_resource"}
    ]
  }
}|


@section[#:tag "other-workers"]{Windows and OS X Workers}

For Windows and OS X, a primitive Garden backend is available, called
@hyperlink["http://github.com/vito/houdini"]{Houdini}.

@margin-note{
  Containers running via Houdini are not isolated from each other. This is
  much less safe than the Linux workers, but will do just fine if you trust
  your builds.

  A proper @hyperlink["https://github.com/cloudfoundry-incubator/garden-windows"]{Garden Windows}
  implementation is in the works.
}

To set up Houdini, download the appropriate binary from its
@hyperlink["https://github.com/vito/houdini/releases/latest"]{latest GitHub
release}.

By default, Houdini will place container data in @code{./containers}
relative to wherever Houdini started from. To change this location, or see
other settings, run Houdini with @code{--help}.


@section[#:tag "linux-workers"]{Out-of-band Linux Workers}

A typical Concourse deployment includes Garden Linux, which is what provides
the default Linux workers.

If you're deploying with BOSH, all worker configuration is automatic, and
you can dynamically grow and shrink the worker pool just by changing the
instance count and redeploying.

However, sometimes your workers can't be automated by BOSH. Perhaps you have
some managed server, and you want to run builds on it.

To do this, you'll probably want to provision Garden Linux to your machine
manually. This can be done with Vagrant via the
@hyperlink["https://github.com/tknerr/vagrant-managed-servers"]{Vagrant
Managed Servers} provider and the
@hyperlink["https://github.com/cppforlife/vagrant-bosh"]{Vagrant BOSH}
provisioner.


@section[#:tag "gate"]{Registering workers with Concourse}

Worker health checking and registration is automated by a tiny component
called @hyperlink["https://github.com/concourse/gate"]{Gate}.

Gate continuously pings the Garden server's API and registers it with the
Concourse API. To run Gate, download the version for your platform from
@hyperlink["https://github.com/concourse/gate/release"]{Gate's GitHub
releases}, and run it like so:

@codeblock|{
  gate \
    -atcAPIURL="http://my.atc.com" \
    -gardenAddr=127.0.0.1:7777 \
    -platform=your-platform \
    -tags=a,b,c
}|

The @code{-atcAPIURL} flag should be the UR, to your Concourse deployment's
web/API server, including any basic auth.

The @code{-gardenAddr} flag specifies the address of the Garden server to
advertise to the ATC. It will be continuously health checked, so it must
also be reachable by the Gate.

The @code{-platform} flag is the platform to advertise for your worker. This
determines which tasks get placed on the worker. Standard names are
@code{linux}, @code{darwin}, and @code{linux}.

The optional @code{-tags} flag specifies a comma-separated list of tag
names. If specified, only tasks with a matching (sub)set of tags will be run
on the worker.

An additional @code{-resourceTypes} flag can be specified, to provide a list
of resource types supported by the worker. This value is provided as a
JSON-formatted list of objects specifying the @code{name} of the resource
type and @code{image} to use for its containers.

Note that there is currently no auth between Concourse and its workers.
Until this is implemented, you should probably lock down the Garden server's
listen addresses to @code{127.0.0.1} and reach them via SSH tunnels, or have
them all running within a private network (e.g. an AWS VPC).


@inject-analytics[]
