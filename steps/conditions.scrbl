#lang scribble/manual

@(require "../common.rkt")

@title[#:version version #:tag "step-conditions"]{@code{conditions}: conditionally perform a step}

Any step can be made conditional by adding the @code{conditions} attribute
to it.

@defthing[conditions [string]]{
  @emph{Optional. Default @code{[success]}.} The conditions under which to
  perform this step.

  @code{[failure]} causes the step to be performed if the previous step's
  task(s) fail, while @code{[success, failure]} means the step will be
  performed regardless of the result.
}

The following will perform the second task only if the first one fails:

@codeblock|{
plan:
  - task: unit
    file: unit.yml
  - conditions: [failure]
    task: alert
    file: alert.yml
}|

If the condition does not match, the conditional step, and all subsequent
steps, are not executed.

If you have multiple conditions you'd like to check, you can wrap them in an
@seclink["aggregate-step"]{@code{aggregate} step}:

@codeblock|{
plan:
  - task: unit
    file: unit.yml
  - aggregate:
      - conditions: [success]
        task: update-status
        file: update-status.yml
        params:
          status: good
      - conditions: [failure]
        task: update-status
        file: update-status.yml
        params:
          status: bad
}|

This will attempt both steps in series, and continue runing the steps after
the @code{aggregate} step regardless.

@inject-analytics[]
