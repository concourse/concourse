#lang concourse/docs

@(require "../common.rkt")

@title[#:version version #:tag "do-step"]{@code{do}@aux-elem{: run steps in series}}

@defthing[do [step]]{
  Simply performs the given steps serially, with the same semantics as if they
  were at the top level step listing.

  This can be used to perform multiple steps serially in the branch of an
  @seclink["aggregate-step"]{@code{aggregate}} step:

  @codeblock|{
  plan:
  - aggregate:
    - task: unit
    - do:
      - get: something-else
      - task: something-else-unit
  }|
}

@inject-analytics[]
