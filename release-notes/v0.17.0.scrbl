#lang scribble/manual

@title[#:style '(quiet unnumbered)]{v0.17.0}

The ATC server is no longer a singleton, though scaling up doesn't yield much
of a performance boost yet. (Perhaps it will after
@hyperlink["https://www.pivotaltracker.com/story/show/81247410"]{#81247410}.)

This release should be fully backwards-compatible. A lot of implementation
detail changed, and a lot of work was put in to make everything a smooth
transition. Concourse is not 1.0 yet, but it's better to think about these
things before people actually use it.

@itemlist[
  @item{
    The communication between the Turbine (workers) and the ATC (web server) is
    no longer done via callbacks. Instead, the ATC pulls down a build's event
    stream via
    @hyperlink["http://en.wikipedia.org/wiki/Server-sent_events"]{Server-Sent
    Events}.

    This means you can remove the @code{atc.callbacks_shared_secret} property
    from your deployment.
  }

  @item{
    In-flight builds are eventually consistent. When started, the Turbine
    they're running on is remembered, and the ATC's job is to keep track of
    this, by pulling down its event stream.

    Each ATC periodically sweeps the database for running builds and phones
    home to their Turbine to process and save the event stream.

    If the Turbine returns a @code{404} for a build, the ATC marks the build
    as errored. This can happen if e.g. the build's Turbine VM died and got
    resurrected.
  }

  @item{
    In a cluster, any ATC instance can now serve build logs. Previously,
    fanning-out build logs to clients was done in-memory, which made the ATC a
    singleton. With the new SSE model, every ATC either proxies to an in-flight
    build's Turbine, or reads all events from the database.
  }

  @item{
    As a result of the switch to SSE and the necessary architectural changes,
    streaming logs is no longer prone to locking up due to slow consumers and
    dead connections.
  }

  @item{
    Build events are stored in a separate table, one row per event, rather than
    as a single gigantic @code{text} column.

    @emph{This is a very expensive migration.} Expect downtime if you have a
    ton of builds, as they're all converted into the new format.
  }

  @item{
    Various minor UI tweaks; the Abort button has moved, and the handling of
    @code{\r} characters in build logs is improved.
  }
]

@section[#:style '(quiet unnumbered)]{Notes on upgrading}

@itemlist[
  @item{
   The database migration that transforms the build logs into the new table
   format can take a @emph{very} long time if you have a lot of builds.

   During the deploy, your web servers will be down for a while as they run the
   migrations on startup.
  }

  @item{
    Unfortunately, logs of in-flight builds will be truncated after the deploy
    completes. This is because the Turbine comes up and does not have an event
    ID to start emitting from, so it starts at 0, which already exists for the
    build, so ATC skips it. Logs saved before the deploy are unaffected.

    The in-flight build's events will still be processed, just not persisted.
    So, they will still go green or red, and their outputs saved; you just won't
    see any more logs for the build.

    Don't expect these kinds of things to happen after 1.0; this was a sacrifice
    made to not grow complexity/baggage early on as Concourse is still in
    "limited release".
  }
]


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
