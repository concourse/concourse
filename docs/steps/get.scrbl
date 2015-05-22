#lang concourse/docs

@(require "../common.rkt")

@title[#:version version #:tag "get-step"]{@code{get}@aux-elem{: fetch a resource}}

Fetches a resource, making it available to subsequent steps via the given
name.

For example, the following plan fetches a version number via the
@code{semver} resource, bumps it to the next release candidate, and
@seclink["put-step"]{@code{put}}s it back.

@codeblock["yaml"]|{
plan:
- get: version
  params:
    bump: minor
    rc: true
- put: version
  params:
    version: version/number
}|

@defthing[get string]{
  @emph{Required.} The logical name of the resource being fetched. This name
  satisfies logical inputs to a @seclink["tasks"]{Task}, and may be referenced
  within the plan itself (e.g. in the @code{file} attribute of a
  @seclink["task-step"]{@code{task}} step).
}

@defthing[resource string]{
  @emph{Optional. Defaults to @code{name}.} The resource to fetch, as
  configured in @seclink["configuring-resources"]{@code{resources}}.
}

@defthing[passed [string]]{
  @emph{Optional.} When specified, only the versions of the resource that
  made it through the given list of jobs will be considered when triggering
  and fetching.

  Note that if multiple @code{get}s are configured with @code{passed}
  constraints, all of the mentioned jobs are correlated. That is, with the
  following set of inputs:

  @codeblock["yaml"]|{
  plan:
  - get: a
    passed: [a-unit, integration]
  - get: b
    passed: [b-unit, integration]
  - get: x
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
  resource. Refer to the resource type's documentation to see what it
  supports.
}

@defthing[trigger boolean]{
  @emph{Optional. Default @code{false}.} Set to @code{true} to auto-trigger
  new builds of the plan's job whenever this step has new versions available,
  as specified by the @code{resource} and any @code{passed} constraints.

  Otherwise, if no @code{get} steps set this to @code{true}, the job can only
  be manually triggered.
}

@inject-analytics[]
