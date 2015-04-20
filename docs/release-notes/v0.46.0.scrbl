#lang scribble/manual

@title[#:style '(quiet unnumbered)]{v0.46.0}

@itemlist[
  @item{
    Jobs can now be configured with @code{serial_groups}, which can be used to
    ensure multiple jobs do not run their builds concurrently. See
    @secref{configuring-jobs} for more information.
  }

  @item{
    Jobs can now be paused. This prevents newly created builds from running
    until the job is unpaused.

    To pause a job, go to its page, which is now accessible by clicking the
    job name when viewing a build, and click the pause button next to its name
    in the header.
  }

  @item{
    The abort button now aborts asynchronously, and also works when aborting
    one-off builds.
  }

  @item{
    If multiple template variables are not bound when configuring via a
    pipeline template, all of their names are printed in the error, rather
    than just the last one.
  }

  @item{
    Resource @code{source} and @code{params} can now contain arbitrarily
    nested configuration.
  }

  @item{
    We've upgraded D3, which now does smoother zooming when double-clicking
    or double-tapping the pipeline view.
  }

  @item{
    The 'started' indicator in the legend now does its little dance once again.
  }
]
