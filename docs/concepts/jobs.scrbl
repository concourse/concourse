#lang scribble/manual

@(require "../common.rkt")

@title[#:version version #:tag "jobs"]{Jobs}

A job is a specification of a @seclink["builds"]{build} to run, combined with
inputs to pull down, and outputs to push up upon the build's successful
completion. Common jobs would be configured to run unit tests, or integration
tests against multiple inputs.

@section{Inputs}

New builds of the job will automatically trigger when any of its inputs change.
Jobs can be interconnected by applying @code{passed} constraints to the inputs
resources - this is the crucial piece that allows pipelines to function.

Inputs can be thought of as the arguments to a function, your build.
Concourse's job is to find which sets of arguments are OK, and which sets of
arguments are not OK.

The notion of a pipeline is introduced when wanting to restrict the potential
set of arguments to ones that are more likely to work, as their values progress
through a sequence of successful builds.

For example, if the latest version of a component causes its unit tests to
fail, you don't want the pipeline to propagate that version to integration tests
or a deploy to prod.

A job's set of inputs can be configured to apply these constraints via the
@code{passed} option. This is described below, along with the rest of the
possible configuration.

@section{Outputs}

When a build succeeds, all of the resource versions used as inputs are
implicitly recorded as outputs of the job.

A job may however configure explicit outputs, which add to the output set,
overriding existing implicit versions.

For example, a job may pull in a repo that contains a @code{Dockerfile}, and
push a Docker image when its tests go green:

@codeblock|{
jobs:
  - name: git-resource-image
    build: git-resource/unit-tests.yml
    inputs:
      - resource: git-resource
    outputs:
      - resource: git-resource-image
        params:
          build: git-resource
}|

When builds complete, all outputs are executed in parallel (as they should have
no inter-relationships). If any outputs fail to execute, the build errors
(overriding the otherwise successful status).

Outputs can be used to do all kinds of things, from bumping version numbers, to
pushing to repos, to marking tasks as finished in your issue tracking system.
These are all modeled as @seclink["resources"]{resources}.
