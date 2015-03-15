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

@inject-analytics[]
