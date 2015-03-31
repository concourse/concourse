#lang scribble/manual

@(require "../common.rkt")

@title[#:style 'toc #:version version #:tag "configuring-jobs"]{@code{jobs}@aux-elem{: Plans to execute against resources}}

@seclink["jobs"]{Jobs} determine the @emph{actions} of your pipeline, how
resources progress through it, and how everything is visualized. They are
listed under the @code{jobs} key in the pipeline configuration.

The following example defines a simple unit-level job that will trigger
whenever new code arrives at the @code{concourse} resource:

@codeblock|{
jobs:
- name: atc-unit
  plan:
  - get: concourse
  - task: unit
    file: concourse/ci/atc.yml
}|

Each configured job consists of the following attributes:

@defthing[name string]{
  @emph{Required.} The name of the job. This should be short; it will show up
  in URLs.
}

@defthing[serial boolean]{
  @emph{Optional. Default @code{false}.} If set to @code{true}, builds will
  queue up and execute one-by-one, rather than executing in parallel.
}

@defthing[public boolean]{
  @emph{Optional. Default @code{false}.} If set to @code{true}, the build log
  of this job will be viewable by unauthenticated users. Unauthenticated users
  will always be able to see the inputs, outputs, and build status history of a
  job. This is useful if you would like to expose your pipeline publicly without
  showing sensitive information in the build log.
}

@defthing[plan [step]]{
  @emph{Required.} The @seclink["build-plans"]{Build Plan} to execute.
}

@inject-analytics[]
