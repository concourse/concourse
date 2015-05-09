#lang concourse/docs

@title[#:style '(quiet unnumbered)]{v0.49.0}

@margin-note{
  Run @seclink["fly-sync"]{@code{fly sync}} to upgrade Fly after deploying
  v0.49.0!
}

@itemlist[
  @item{
    @secref{fly-execute} once again works with https-only deployments. This
    was a regression introduced in @secref{v0_47_0} as part of the
    highly-available @code{execute} work.
  }

  @item{
    Aborting a not-yet-started build now correctly updates the UI to show that
    the build is aborted. Previously you had to refresh.
  }
]
