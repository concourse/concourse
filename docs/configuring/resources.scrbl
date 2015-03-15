#lang scribble/manual

@(require "../common.rkt")

@title[#:style 'toc #:version version #:tag "configuring-resources"]{@code{resources}: Objects flowing through the pipeline}

Resources define the objects that are going to be used for jobs in the pipeline.
They are continuously checked for new versions.

The following example defines a resource representing Concourse's BOSH release
repository:

@codeblock|{
resources:
- name: concourse
  type: git
  source:
    uri: https://github.com/concourse/concourse.git
}|

Any time commits are pushed, the resource will detect them and save off new
versions of the resource. Once they're all saved, any dependent jobs will be
triggered. The resource can also be updated via a
@seclink["put-step"]{@code{put} step} in the pipeline's jobs.

Resources have the following properties:

@defthing[name string]{
  @emph{Required.} The name of the resource. This should be short and simple.
  This name will be referenced by @seclink["build-plans"]{build plans} of jobs
  in the pipeline.
}

@defthing[type string]{
  @emph{Required.} The type of the resource. Each worker advertises a mapping
  of @code{resource-type -> container-image}; @code{type} corresponds to the
  key in the map.

  To see what resource types your deployment supports, check
  @code{/api/v1/workers}. A random worker that advertises the given type will
  be chosen.
}

@defthing[source object]{
  @emph{Optional.} The location of the resource. This varies
  by resource type, and is a black box to Concourse; it is simply passed to
  the resource at runtime.

  To use @code{git} as an example, the source may contain the repo URI, which
  branch, and a private key to use when pushing/pulling.

  By convention, documentation for each resource type's configuration is
  in each implementation's @code{README}.

  You can find the implementations of resource types provided with Concourse
  at the @hyperlink["https://github.com/concourse?query=-resource"]{Concourse
  GitHub organization}.
}

@inject-analytics[]
