#lang scribble/manual

@(require "../common.rkt")

@title[#:version version #:tag "put-step"]{@code{put}: update a resource}

Pushes to the given @seclink["resources"]{Resource} using the state from the
preceding steps, if available.

For example, the following plan fetches a repo using
@seclink["get-step"]{@code{get}} and push it to another repo (assuming
@code{repo-a} and @code{repo-b} are defined as @code{git} resources):

@codeblock|{
plan:
  - get: repo-a
  - put: repo-b
    params:
      repository: ./
}|

@defthing[put string]{
  @emph{Required.} The logical name of the resource being pushed.
}

@defthing[resource string]{
  @emph{Optional. Defaults to @code{name}.} The resource to update,
  as configured in @seclink["configuring-resources"]{@code{resources}}.
}

@defthing[params object]{
  @emph{Optional.} A map of arbitrary configuration to forward to the
  resource. Refer to the resource type's documentation to see what it
  supports.
}

@inject-analytics[]
