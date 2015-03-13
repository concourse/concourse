#lang scribble/manual

@title[#:style '(quiet unnumbered)]{v0.39.0}

@margin-note{
  Run @seclink["fly-sync"]{@code{fly sync}} to upgrade Fly after deploying
  v0.39.0!
}

@itemlist[
  @item{
    The biggest and best feature of this release is support for arbitrary build
    plans. Jobs no longer need to take the form of inputs to a build to
    outputs (although this is still possible). Jobs can now run steps in
    parallel, aggregate inputs together, and push to outputs in the middle of a
    build. The applications and possibilities are too numerous to list. So I'm
    not going to bother.

    Since there is now more than one stage to @code{hijack} into we've added
    new flags for the step name (@code{-n}) and step type (@code{-t}). You can
    use these to gain shell access to any step of your build.

    For more information on how you can start using these VISIT THESE REALLY
    COOL DOCS. We've found a 42.73% increase in happiness from people who use
    this feature.
  }

  @item{
    In related news, we needed to upgrade the UI to support all of these
    wonderful new flows so we'vw spruced up the build log page a little. There
    are now individual success/failure markers for each stage and stages that
    are not-interesting (successful) will automatically collapse. There are
    also little icons. Totally rad.
  }

  @item{
    A few of you noticed that having multiple ignore paths in the
    @code{git-resource} wasn't really working properly. Well, we've fixed
    that.
  }
]
