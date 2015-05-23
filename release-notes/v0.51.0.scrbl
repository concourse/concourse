#lang concourse/docs

@title[#:style '(quiet unnumbered)]{v0.51.0}

This release contains @emph{backwards-incompatible} changes that clean up the
semantics of @secref{put-step} and @secref{get-step} within build plans.

@itemlist[
  @item{
    In a given build plan, @emph{all} @secref{get-step} steps are considered
    inputs to the plan's outer job, and have their version determined before
    the build starts running. Previously only the "firstmost" @code{get}s (or
    ones with @code{passed} constraints) were considered inputs, which was
    pretty confusing.

    As part of this, @code{trigger} now defaults to @code{false}, rather than
    @code{true}.

    To auto-trigger based on a @secref{get-step} step in your plan, you must
    now explicitly say @code{trigger: true}.

    So, a build plan that looks like...:

    @codeblock["yaml"]{
    - aggregate:
      - get: a
      - get: b
        trigger: false
    - get: c
    }

    ...would be changed to...:

    @codeblock["yaml"]{
    - aggregate:
      - get: a
        trigger: true
      - get: b
    - get: c
    }

    ...with one subtle change: the version of @code{c} is determined before
    the build starts, rather than fetched arbitrarily in the middle.
  }

  @item{
    All @secref{put-step} steps now imply a @secref{get-step} of the created
    resource version. This allows build plans to produce an artifact and use
    the artifact in later steps.

    These implicit @secref{get-step} steps are displayed differently in the UI
    so as to not be confused with explicit @secref{get-step} steps.

    So, a build plan that looks like...:

    @codeblock["yaml"]{
    - get: a
    - put: b
    - get: b
    - put: c
      params: {b/something}
    }

    ...would be changed to...:

    @codeblock["yaml"]{
    - get: a
    - put: b
    - put: c
      params: {b/something}
    }

    The main difference being that this now @emph{guarantees} that the version
    of @code{b} that @code{c} uses is the same version that was created.

    Note that, given the first change, this is the only way for new versions
    to appear in the middle of a build plan.
  }

  @item{
    The rendering of the pipeline graph has been improved; the algorithm now
    closes gaps between jobs by shifting them as far to the right as possible.
    This prevents input lines from meandering across the UI.
  }

  @item{
    The
    @hyperlink["https://github.com/concourse/docker-image-resource"]{Docker
    Image resource} now supports tagging images from a file, rather than
    a fixed tag. This is useful for creating images based on upstream
    versioned artifacts.
  }

  @item{
    The
    @hyperlink["https://github.com/concourse/docker-image-resource"]{Docker
    Image resource} can now be used to push to a private registry with no
    auth.
  }

  @item{
    The
    @hyperlink["https://github.com/concourse/docker-image-resource"]{Docker
    Image resource} can now be configured with @code{server_args}, to inject
    arguments to the daemon, e.g. @code{--insecure-registry}.
  }
]
