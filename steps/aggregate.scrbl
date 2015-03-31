#lang scribble/manual

@(require "../common.rkt")

@title[#:version version #:tag "aggregate-step"]{@code{aggregate}@aux-elem{: run steps in parallel}}

@defthing[aggregate [step]]{
  Performs the given steps in parallel.

  If any sub-steps in an aggregate result in an error, the aggregate step as a
  whole is considered to have errored.

  Similarly, when aggregating @seclink["task-step"]{@code{task}} steps, if any
  @emph{fail}, the aggregate step will fail. This is useful for build matrixes:

  @codeblock|{
  plan:
  - get: some-repo
  - aggregate:
    - task: unit-windows
      file: some-repo/ci/windows.yml
    - task: unit-linux
      file: some-repo/ci/linux.yml
    - task: unit-darwin
      file: some-repo/ci/darwin.yml
  }|

  The @code{aggregate} step is also useful for performing arbitrary steps in
  parallel, for the sake of speeding up the build. It is often used to fetch
  all dependent resources together:

  @codeblock|{
  plan:
  - aggregate:
    - get: component-a
    - get: component-b
    - get: integration-suite
  - task: integration
    file: integration-suite/task.yml
  }|
}

@inject-analytics[]
