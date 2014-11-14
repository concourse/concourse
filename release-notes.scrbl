#lang scribble/manual

@(require "common.rkt")

@title[#:version version #:tag "release-notes"]{Release Notes}

@section[#:style '(quiet unnumbered)]{v0.22.0}

Fixed a minor regression in the pipeline UI, which caused arrows between jobs
and their outputs to be colored incorrectly. This was released shortly after
@code{v0.21.0}, which contains more interesting stuff.


@section[#:style '(quiet unnumbered)]{v0.21.0}

Backwards-compatible, more polish.

@itemlist[
  @item{
    The pipeline view on the main page now auto-updates every 5 seconds.
  }

  @item{
    The pipeline rendering logic has been improved. Common gateway nodes are now
    shared (i.e. if there are fan-ins from the same sources), which helps for
    some more complicated pipeline configurations. The pipeline is also no
    longer randomized.
  }

  @item{
    There are new read-only APIs for getting job and resource metadata from a
    deployment. Also, all objects exposed through the API now contain their URL.
  }

  @item{
    The Concourse release now bundles a component called @code{blackbox}. This
    new component ships the system logs off to a syslog sink, like
    @hyperlink["https://papertrailapp.com"]{Papertrail}.

    To enable Blackbox, colocate the @code{blackbox} job template on each BOSH
    job, and configure @code{blackbox.destination.address} to point to the
    @code{ip:port} of your syslog sink.
  }

  @item{
    Fly will now automatically reconnect if the connection breaks while
    streaming logs.
  }

  @item{
    All Concourse components no longer run as @code{root}. They run as the
    unprivileged @code{vcap} user provided by BOSH.
  }

  @item{
    Errored builds no longer render a corrupt duration.
  }

  @item{
    A deadlock in ATC's resource checking has been fixed. This deadlock would
    prevent the BOSH job from updating; if your deploy hangs you may have to SSH
    into the VM and manually kill ATC.
  }

  @item{
    Fixed up some of the locking logic in ATC; it would previously fail early
    due to a race condition. This could lead to things being scheduled
    arbitrarily later than they should have been, if you were unlucky enough to
    have it happen multiple times in a row.
  }
]


@section[#:style '(quiet unnumbered)]{v0.20.0}

Backwards-compatible, with more high-availability progress, and now
reconfigurable via the API.

This also marks the first release available as a pre-built Vagrant box. For
a fast local development bootstrap, execute the following anywhere (assuming
Vagrant is installed):

@codeblock|{
vagrant init concourse/lite
vagrant up
export ATC_URL=http://192.168.100.4:8080
}|

And then @code{fly} away!

Full release notes:

@itemlist[
  @item{
    The pipeline can now be reconfigured via ATC's API. Existing deployments
    will remain working, but cannot be configured through their API, as their
    source of truth is still the BOSH deployment manifest.

    To switch to being configured via the API, first create a single @code{.yml}
    file containing your current configuration. This can be done by extracting
    the @code{pipeline} config from your existing manifest, or by running
    @code{fly configure} with no flags.

    Once you have your pipeline as a standalone config file, apply it by running
    @code{fly configure -c <path/to/pipeline.yml}. To reconfigure, edit the file
    and run the same command.
  }

  @item{
    The ATC's locking for performing tasks like scheduling builds, checking for
    resources, and tracking build progress is now fine-grained. This means it's
    more likely for the workload to spread across a cluster of ATCs, but the
    distribution is still nowhere near even.
  }

  @item{
    The Git resource now supports blacklisting/whitelisting files, and
    configuring which submodules get synced. These features are a big help when
    there's one big repository being threaded through a pipeline for different
    purposes at different stages.

    For more information, see the
    @hyperlink["https://github.com/concourse/git-resource/blob/46c88f4556dc3d4f3d602dbec34b3aa69fa41c86/README.md"]{Git Resource's README.md}.
  }
]



@section[#:style '(quiet unnumbered)]{v0.19.0}

The graph library used to render the pipeline has been updated, and along
the way the visualization of pipeline groups has been improved.


@section[#:style '(quiet unnumbered)]{v0.18.0}

The version of Consul was rolled back to @code{0.4.0} from @code{0.4.1}, as
it was noticed that @code{0.4.1} broke single-server deployments.


@section[#:style '(quiet unnumbered)]{v0.17.0}

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

@subsection[#:style '(quiet unnumbered)]{Notes on upgrading}

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
