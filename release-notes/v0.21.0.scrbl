#lang scribble/manual

@title[#:style '(quiet unnumbered)]{v0.21.0}

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
