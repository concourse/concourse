#lang concourse/docs

@title[#:style '(quiet unnumbered)]{v0.48.0}

@margin-note{
  Run @seclink["fly-sync"]{@code{fly sync}} to upgrade Fly after deploying
  v0.48.0!
}

@itemlist[
  @item{
    Pipelines can now be paused via the web UI. This acts like pausing all
    jobs and resources in the pipeline; checking will be disabled, and new
    builds will not be scheduled.
  }

  @item{
    Pipelines can be destroyed via @secref{fly-destroy-pipeline}. This
    destroys all data contained in the pipeline, including all build history
    and resource versions.
  }

  @item{
    Pipelines can be re-ordered by dragging them around in the sidebar.
  }

  @item{
    The main view (@code{/}) now shows the first pipeline in order, rather
    than @code{main}.
  }

  @item{
    Fly now copes with an extra trailing slash in the target URL. End of an
    era.
  }

  @item{
    @secref{fly-intercept} no longer defaults the @code{-t} flag to
    @code{task}, and instead leaves it up to the rest of the flags to
    disambiguate. This means you can just run @code{-n some-input} and it'll
    probably "just work," unless you have another step called
    @code{some-input}. In which case you can just pass @code{-t} to
    disambiguate.
  }

  @item{
    We have removed the animation from the pipelines sidebar. It was a bit too
    cool compared to the rest of the UI.
  }

  @item{
    You can now click the area around the hamburger, instead of having to
    sharpshoot the hamburger.
  }

  @item{
    The @hyperlink["https://github.com/concourse/git-resource"]{Git resource}
    now copes with a repository being @code{push -f}ed.
  }
]
