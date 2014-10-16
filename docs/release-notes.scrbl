#lang scribble/manual

@title[#:tag "release-notes"]{Release Notes}

@section[#:style '(quiet unnumbered)]{v0.16.0}

Yet more polish, with a few backwards-incompatible changes that are better made
before 1.0.

@itemlist[
  @item{
    A build can now specify an explicit set of inputs as part of its config. If
    the build is ever triggered without the set of named inputs satisfied, it
    will error.

    The @code{paths} configuration is now done in terms of @code{inputs}.

    @emph{To transition}: replace @code{paths} with a set of @code{inputs} with
    @code{path} values configured on them.

    For example:

    @codeblock|{
      paths:
        some-input: some/path
    }|

    ...becomes:

    @codeblock|{
      inputs:
        - name: some-input
          path: some/path
    }|
  }

  @item{
    @code{dont_check} renamed: the @code{dont_check} property on job inputs has
    been renamed to @code{trigger}, and the value is now the opposite of what
    it used to be.

    This was done because the previous name was misleading; every resource is
    @emph{always} checked; this configuration merely configures whether newly
    detected versions for the input should trigger new builds.

    @emph{To transition}: replace @code{dont_check: true} with @code{trigger:
    false}.
  }

  @item{
    The @code{on} property of job outputs has been renamed.

    YAML parses @code{on}, @code{off}, @code{yes}, and @code{no} as boolean
    values.  This made the @code{on: [success, failure]} output config have to
    be @code{"on": ...}, which is awkward. The new name is @code{perform_on}.

    @emph{To transition}: replace @code{on: ...} with @code{perform_on: ...}.
  }

  @item{
    Build logs now properly handle the @code{\r} escape sequence, which many
    command-line tools use for showing progress indicators, etc. So, any tools
    that emit progress bars (@code{curl}, @code{bosh}) will now have much
    prettier output.
  }

  @item{
    The @code{git} resource can now be configured to attempt rebasing before
    pushing. This is helpful when CI is pushing to the same repo that other
    jobs or humans will be pushing to, and the update is assumed to be easily
    mergeable if done out-of-order.

    This is configured by specifying @code{rebase: true} in the output params.
  }

  @item{
    Some of the pipeline grouping UI logic has been cleaned up: jobs will
    always show their immediate upstream/downstream inputs, even if they're
    jobs in other groups. This is much clearer than omitting them.

    Also, jobs with no configured group will no longer appear in every group.
    This was a conservative first step, but it seems like it should be easy to
    notice a missing job while configuring groups, rather than be confused
    about a job showing up on every page.

    Ultimately an "orphaned" page may be introduced to show these jobs.
  }

  @item{
    There is now a legend shown on the pipeline view, which shows what each
    color means.
  }

  @item{
    Viewing a build now shows when it started, when it ended, and the duration
    of the build. The UI here is still in the works, but the information is at
    least being collected.
  }

  @item{
    The version of Consul, which Concourse uses for service discovery, has been
    bumped up to v0.4 from v0.3.1. This should be backwards-compatible, but
    it's still early in Consul's lifecycle, so if you have any issues that look
    like two nodes stopped talking to each other, let us know in #concourse on
    Freenode.
  }
]
