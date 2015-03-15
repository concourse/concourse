#lang scribble/manual

@(require "../common.rkt")

@title[#:version version #:tag "aggregate-step"]{@code{aggregate}@aux-elem{: run steps in parallel}}

Performs all of the given steps in parallel, aggregating their resulting
file tree for subsequent steps.

If you have a @seclink["tasks"]{Task} that expects multiple inputs, you
would precede it with an @code{aggregate} containing a
@seclink["get-step"]{@code{get}} step for each input:

@codeblock|{
plan:
  - aggregate:
      - get: component-a
      - get: component-b
      - get: integration-suite
  - task: integration
    file: integration-suite/task.yml
}|

@defthing[aggregate [step]]{
  Configures steps to execute in parallel.

  For each step, the resulting data will be collected and made available to
  the steps following the aggregate, under a subdirectory named after the
  branch's step.

  That is, with the following aggregate:

  @codeblock|{
    aggregate:
      - get: component-a
      - get: component-b
  }|

  The subsequent steps will run with the following data as the input:

  @codeblock|{
    component-a/...
    component-b/...
  }|

  Where @code{...} is shorthand for whatever data was collected by the
  respective @code{get} steps.
}

@inject-analytics[]
