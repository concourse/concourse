#lang concourse/docs

@title[#:style '(quiet unnumbered)]{v0.47.0}

@margin-note{
  Run @seclink["fly-sync"]{@code{fly sync}} to upgrade Fly after deploying
  v0.47.0!
}

@itemlist[
  @item{
    A single Concourse deployment can now be configured with multiple
    pipelines. See the new docs for @secref{fly-configure} for more information.
  }

  @item{
    Fly now supports saving targets by name. See @secref{fly-save-target} for more information.
  }

  @item{
    You can now run @secref{fly-execute} against a deployment with multiple
    ATCs. Previously this would fail when downloading the inputs.

    With this change you should be able to scale up ATCs for high availability
    and everything should "just work." They already balance work across each
    other, this was just the last remaining thing preventing it from being
    useful.
  }

  @item{
    The Fly CLI no longer defaults to running @code{execute}. This feature
    prevented us from implementing proper global flag semantics, which made
    passing flags confusing in places. If @code{fly} is run with no arguments,
    it will now print out the help text.
  }

  @item{
    One-off builds are now scheduled immediately. Previously we had forgotten
    to kick them off, which resulted in it being picked up lazily, causing an
    up to 10 second delay. This should make @code{fly execute} feel much
    snappier.
  }

  @item{
    The resource view looks a bit better.
  }
]
