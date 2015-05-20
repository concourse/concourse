#lang concourse/docs

@(require "../common.rkt")

@title[#:version version #:tag "put-step"]{@code{put}@aux-elem{: update a resource}}

Pushes to the given @seclink["resources"]{Resource}. All artifacts collected
during the plan's execution will be available in the working directory.

For example, the following plan fetches a repo using
@seclink["get-step"]{@code{get}} and pushes it to another repo (assuming
@code{repo-a} and @code{repo-b} are defined as @code{git} resources):

@codeblock["yaml"]|{
plan:
- get: repo-a
- put: repo-b
  params:
    repository: repo-a
}|

When the @code{put} succeeds, the produced version of the resource will be
immediately fetched via an implicit @secref{get-step} step. This is so that
later steps in your plan can use the artifact that was produced. The source
will be available under whatever name @code{put} specifies, just like as with
@code{get}.

So, if the logical name (whatever @code{put} specifies) differs from the
concrete resource, you would specify @code{resource} as well, like so:

@codeblock["yaml"]|{
plan:
- put: resource-image
  resource: docker-image-resource
}|

@defthing[put string]{
  @emph{Required.} The logical name of the resource being pushed. The pushed
  resource will be available under this name after the push succeeds.
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
