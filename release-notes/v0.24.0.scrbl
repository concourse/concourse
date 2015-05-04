#lang concourse/docs

@title[#:style '(quiet unnumbered)]{v0.24.0}

@itemlist[
  @item{
    More improvements to pipeline rendering, mainly affecting configurations
    with groups and fan-in nodes.
  }

  @item{
    Inputs to jobs can be hidden from the pipeline with @code{hidden: true}.
    This is useful for common inputs (e.g. repositories containing credentials)
    that feed into many jobs but aren't really interesting enough to actually
    show. These tend to clutter up the flow, and can now be hidden.
  }

  @item{
    Components no longer drain indefinitely. This was a discipline kept
    throughout development to catch deadlocked code paths, but not a great
    practice for production deployments.
    
    They will now be given time to safely shut down, and then kill -9'd. All
    relevant code paths were audited and deemed to be interruptible, or fixed.

    As a result of this deploys should never hang again.
  }
]
