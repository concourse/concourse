#lang scribble/manual

@(require "common.rkt")

@title[#:version version #:tag "pipelines"]{Pipelines}

Together, jobs, resources, and builds form a pipeline.

Here's an example of a fairly standard unit → integration → deploy pipeline:

@image[#:suffixes '(".svg" ".png") "images/example-pipeline"]{Example Pipeline}

Above, the black boxes are @seclink["resources"]{resources}, and the colored
boxes are @seclink["jobs"]{jobs}, whose color and wiggliness indicates the
status of their current @seclink["builds"]{build}. It is also possible to
@seclink["configuring-groups"]{group} different sections of a pipeline together into
logical collections.

The component responsible for displaying and scheduling this pipeline is called
the ATC (air traffic controller). In a @seclink["deploying-with-bosh"]{BOSH
deployment}, the @code["atc.config"] property specifies the ATC's pipeline
configuration in full.

A pipeline is configured with two sections:
@seclink["configuring-resources"]{@code{resources}} and
@seclink["configuring-jobs"]{@code{jobs}}. For example, the configuration
resulting in the above pipeline is as follows:

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

To learn what the heck that means, read on.


@section[#:tag "configuring-resources"]{@code{resources}}

The @code{resources} section lists all of the potential input sources and
output destinations of the pipeline. For example, the Git repositories for your
project, and the S3 bucket it ships to.

Each configured resource consists of the following attributes:

@defthing[name string]{
  @emph{Required.} The name of the resource. This should be short and simple,
  for example the name of the repo.
}

@defthing[type string]{
  @emph{Required.} The type of the resource. Each worker is configured with a
  static mapping of @code{resource-type -> container-image} on startup;
  @code{type} corresponds to the key in the map.

  For example, declaring @code{git} here will result in the workers using
  @code{docker:///concourse/git-resource} if configured with that mapping.
}

@defthing[source object]{
  @emph{Optional.} The location of the resource. This varies
  by resource type, and is a black box to Concourse; it is simply passed to
  the resource at runtime. For example, this may describe where your Git repo
  lives, and which branch to pull down, and a private key to use for
  pushing/pulling.

  Note that this is fairly open-ended; the documentation for what can be
  included in @code{source} is left to the individual resources.
}


@section[#:tag "configuring-jobs"]{@code{jobs}}

The @code{jobs} section is a list of all "connection points" in the pipeline.
A job configures the superset of a build configuration, describing which
resources to fetch and trigger the build by, and which resources to have as
outputs of a successful build.

Each configured job consists of the following attributes:

@defthing[name string]{
  @emph{Required.} The name of the job. This should be short; it will show up
  in URLs.
}

@deftogether[(@defthing[build string] @defthing[config object])]{
  @emph{Required.} The configuration for the build's running environment.

  @code{build} points at a @code{.yml} file containing the
  @seclink["configuring-builds"]{build config}, which allows this to be tracked
  with your resources. The file is provided by an input resource, so typically
  this value may be @code{resource-name/build.yml}.

  @code{config} can be defined to inline the same configuration as
  @code{build.yml}.

  If both are specified, the value of @code{config} is "merged" into the config
  loaded from the file. This allows build parameters to be specified in the
  build's config file, and overriden in the pipeline (i.e. for passing in
  secret credentials).
}

@defthing[inputs [object]]{
  @emph{Optional.} Resources that should be available to the build. By
  default, when new versions of any of them are detected, a new build of the job
  is triggered.

  A job's @code{inputs} each contain the following configuration:

  @nested[#:style 'inset]{
    @defthing[resource string]{
      @emph{Required.} The resource to pull down, as described in
      @secref{configuring-resources}.
    }

    @defthing[name string]{
      @emph{Optional.} The logical name of this resource, as expected by the
      job's build configuration. If not specified, it defaults to the
      @code{resource} name.
    }

    @defthing[passed [string]]{
      @emph{Optional.} When configured, only the versions of the resource that
      appear as outputs of the given list of jobs will be considered for inputs
      to this job.

      Note that if multiple inputs are configured with @code{passed}
      constraints, all of the mentioned jobs are correlated. That is, with the
      following set of inputs:

      @codeblock|{
      inputs:
      - resource: a
        passed: [a-unit, integration]
      - resource: b
        passed: [b-unit, integration]
      - resource: x
        passed: [integration]
      }|

      This means "give me the versions of @code{a}, @code{b}, and @code{x} that
      have passed the @emph{same build} of @code{integration}, with the same
      version of @code{a} passing @code{a-unit} and the same version of
      @code{b} passing @code{b-unit}."

      This is crucial to being able to implement safe "fan-in" semantics as
      things progress through a pipeline.
    }

    @defthing[params object]{
      @emph{Optional.} A map of arbitrary configuration to forward to the
      resource's @code{in} script.
    }

    @defthing[trigger boolean]{
      @emph{Optional.} Default @code{true}. By default, when any of a job's
      inputs have new versions, a new build of the job is triggered.

      Setting this to @code{false} disables this behavior; the job will only
      trigger via other inputs or by a user manually triggering.
    }
  }
}

@defthing[outputs [object]]{
  @emph{Optional.} Resources that have new versions generated upon successful
  completion of this job's builds. For example, you may want to push commits to
  a different branch, or update code coverage reports, or mark tasks finished.

  A job's @code{outputs} each contain the following configuration:

  @nested[#:style 'inset]{
    @defthing[resource string]{
      @emph{Required.} The resource to update, as described in
      @secref{configuring-resources}.
    }

    @defthing[params object]{
      @emph{Optional.} A map of arbitrary configuration to forward to the
      resource's @code{out} script.
    }

    @defthing[perform_on [string]]{
      @emph{Optional. Default @code{[success]}.} The conditions under which to
      perform this output. For example, @code{[failure]} causes the output to be
      performed if the build fails, while @code{[success, failure]} means the
      output will be performed regardless of the build's result.
    }
  }
}

@defthing[serial boolean]{
  @emph{Optional. Default @code{false}.} If set to @code{true}, builds will
  queue up and execute one-by-one, rather than executing in parallel.
}

@defthing[privileged boolean]{
  @emph{Optional. Default @code{false}.} If set to @code{true}, builds will run
  as @code{root}. This is not part of the build configuration to prevent
  privilege escalation via pull requests. This is a gaping security hole; use
  wisely.
}

@defthing[public boolean]{
  @emph{Optional. Default @code{false}.} If set to @code{true}, the build log
  of this job will be viewable by unauthenticated users. Unauthenticated users
  will always be able to see the inputs, outputs, and build status history of a
  job. This is useful if you would like to expose your pipeline publicly without
  showing sensitive information in the build log.
}

@section[#:tag "configuring-groups"]{@code{groups}}

A pipeline may optionally contain a section called @code{groups}. As more
resources and jobs are added to a pipeline it can become difficult to navigate.
Pipeline groups allow you to group jobs together under a header and have them
show on different tabs in the user interface. Groups have no functional effect
on your pipeline.

A simple grouping for the pipeline above may look like:

@codeblock|{
groups:
  - name: tests
    jobs:
      - controller-mysql
      - controller-postgres
      - worker
      - integration
  - name: deploy
    jobs:
      - deploy
}|

This would display two tabs at the top of the home page: "tests" and "deploy".

For a real world example of how groups can be used to simplify navigation and
provide logical grouping, see the groups used at the top of the page in the
@hyperlink["http://ci.concourse.ci"]{Concourse pipeline}.

Each group allows you to set the following attributes:

@defthing[name string]{
  @emph{Required.} The name of the group. This should be short and simple as
  it will be used as the tab name for navigation.
}

@defthing[jobs [string]]{
  @emph{Optional.} A list of jobs that should appear in this group. A job may
  appear in multiple groups. Neighbours of jobs in the current group will also
  appear on the same page in order to give context of the location of the
  group in the pipeline.
}

@defthing[resources [string]]{
  @emph{Optional.} A list of resources that should appear in this group.
  Resources that are inputs or outputs of jobs in the group are automatically
  added; they do not have to be explicitly listed here.
}
