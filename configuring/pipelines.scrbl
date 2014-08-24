#lang scribble/manual

@title{Pipelines}

Together, jobs, resources, and builds form a pipeline. A pipeline a
configuration of a set of jobs and a set of resources, all interrelated.

Here's an example of a fairly standard unit &rarr; integration &rarr; deploy
pipeline configuration:

@codeblock|{
resources:
  - name: controller
    type: git
    source:
      uri: git@github.com:my-org/controller.git
      branch: master

  - name: worker
    type: git
    source:
      uri: git@github.com:my-org/worker.git
      branch: master

  - name: integration-suite
    type: git
    source:
      uri: git@github.com:my-org/integration-suite.git
      branch: master

  - name: release
    type: git
    source:
      uri: git@github.com:my-org/release.git
      branch: master

  - name: final-release
    type: s3
    source:
      bucket: concourse-releases
      regex: release-(.*).tgz

jobs:
  - name: controller-mysql
    build: controller/ci/mysql.yml
    inputs:
      - resource: controller

  - name: controller-postgres
    build: controller/ci/postgres.yml
    inputs:
      - resource: controller

  - name: worker
    build: worker/build.yml
    inputs:
      - resource: worker

  - name: integration
    build: intregation-suite/build.yml
    inputs:
      - resource: integration-suite
      - resource: controller
        passed: [controller-mysql, controller-postgres]
      - resource: worker
        passed: [worker]

  - name: deploy
    build: release/ci/deploy.yml
    serial: true
    inputs:
      - resource: release
      - resource: controller
        passed: [integration]
      - resource: worker
        passed: [integration]
    outputs:
      - resource: final-release
        params:
          from: release/build/*.tgz
}|

Rendered by Concourse, this would look like:

@image["images/example-pipeline.svg"]{Example Pipeline}

To learn what the heck that means, read on.


@section{Configuring Resources}

Resources are configured as a list of objects under @code{resources} at the top
level, each with the following values:

@itemlist[
  @item{
    @code{name}: @emph{Required.} The name of the resource. This should be short and simple,
    for example the name of the repo.
  }

  @item{
    @code{type}: @emph{Required.} The type of the resource. This maps to a container image
    configured by your workers for the given type.
  }

  @item{
    @code{source}: @emph{Optional.} The location of the resource. This varies by resource
    type, and is a black box to Concourse; it is simply passed to the resource at
    runtime. For example, this may describe where your Git repo lives, and which
    branch to pull down, and a private key to use for pushing/pulling.
  }
]

Note that this is fairly open-ended; the documentation for what can be included
in @code{source} is left to the individual resources.


@section{Configuring Jobs}

A job configures the superset of a build configuration, describing which
resources to fetch and trigger the build by, which resources to have as outputs
of a successful build, and various other knobs.

Jobs are configured as a list of objects under @code{jobs} at the toplevel. Each
object has the following attributes:

@itemlist[
  @item{
    @code{name}: @emph{Required.} The name of the job. This should be short; it will
    show up in URLs.
  }

  @item{
    @code{build} or @code{config}: @emph{Required.} The configuration for the build's running
    environment.  @code{build} points at a @code{.yml} file containing the config, which
    allows this to be tracked with your resources. @code{config} can be defined to
    inline the same configuration.
  }

  @item{
    @code{inputs}: @emph{Optional.} Resources that should be available to the build. By
    default, when new versions of any of them are detected, a new build of the job
    is triggered.
    
    See @secref{Inputs} and @secref{configuring-inputs}.
  }

  @item{
    @code{outputs}: @emph{Optional.} Resources that have new versions generated upon
    successful completion of this job's builds. For example, you may want to push
    commits to a different branch, or update code coverage reports, or mark tasks
    finished.
    
    See @secref{Outputs} and @secref{configuring-outputs}.
  }

  @item{
    @code{serial}: @emph{Optional. Default @code{false}.} If set to @code{true}, builds will queue up
    and execute one-by-one, rather than executing in parallel.
  }

  @item{
    @code{privileged}: @emph{Optional. Default @code{false}.} If set to @code{true}, builds will run
    as @code{root}. This is not part of the build configuration to prevent privilege
    escalation via pull requests. This is a gaping security hole; use wisely.
  }
]


@subsection[#:tag "configuring-inputs"]{Configuring Inputs}

A job's @code{inputs} each contain the following configuration:

@itemlist[
  @item{
    @code{resource}: @emph{Required.} The resource to pull down, as described in @code{resources}
    next to the full @code{jobs} configuration.
  }

  @item{
    @code{passed}: @emph{Optional.} When configured, only the versions of the resource that
    appear as outputs of the given list of jobs will be considered for inputs to
    this job.

    Note that if multiple inputs are configured with @code{passed} constraints, all of
    the mentioned jobs are correlated. That is, with the following set of inputs:

    @codeblock|{
    inputs:
    - resource: a
      passed: [a-unit, integration]
    - resource: b
      passed: [b-unit, integration]
    - resource: x
      passed: [integration]
    }|

    This means "give me the versions of @code{a}, @code{b}, and @code{x} that have passed the
    @emph{same build} of @code{integration}, with the same version of @code{a} passing @code{a-unit}
    and the same version of @code{b} passing @code{b-unit}."

    This is crucial to being able to implement safe "fan-in" semantics as things
    progress through a pipeline.
  }

  @item{
    @code{params}: @emph{Optional.} A map of arbitrary configuration to forward to the
    resource's @code{in} script.
  }

  @item{
    @code{dont_check}: @emph{Optional.} Setting this to @code{true} will ensure that the job is
    not auto-triggered when this input's resource is the only thing that has
    changed.
  }
]


@subsection[#:tag "configuring-outputs"]{Configuring Outputs}

A job's @code{outputs} each contain the following configuration:

@itemlist[
  @item{
    @code{resource}: @emph{Required.} The resource to perform the output on (think push
    target).
  }

  @item{
    @code{params}: @emph{Optional.} A map of arbitrary configuration to forward to the
    resource's @code{out} script.
  }
]
