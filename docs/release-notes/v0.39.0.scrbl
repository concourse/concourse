#lang concourse/docs

@title[#:style '(quiet unnumbered)]{v0.39.0}

@margin-note{
  Run @seclink["fly-sync"]{@code{fly sync}} to upgrade Fly after deploying
  v0.39.0!
}

This release adds a new way of configuring jobs called
@seclink["build-plans"]{Build Plans}.

We'll be removing support for the old style of job configuration in the future;
to automatically migrate your configuration, just run
@seclink["fly-configure"]{@code{fly configure}} against your instance to update
your local configuration after upgrading to this release.

For more details, read on.

@itemlist[
  @item{
    The biggest and best feature of this release is support for arbitrary
    @seclink["build-plans"]{Build Plans}. Jobs no longer need to take the form
    of inputs to a build to outputs (although this is still possible). Jobs can
    now run steps in parallel, aggregate inputs together, and push to outputs
    in the middle of a build. The applications and possibilities are too
    numerous to list. So I'm not going to bother.

    Since there is now more than one stage to @code{hijack} into we've added
    new flags for the step name (@code{-n}) and step type (@code{-t}). You can
    use these to gain shell access to any step of your build.

    For more information on how you can start, see
    @seclink["build-plans"]{Build Plans}. We've found a 43.73% increase in
    happiness from people who use this feature.

    As part of rolling out build plans, we now automatically translate the old
    configuration format to the new plan-based configuration.
  }


  @item{
    We've renamed what was formerly known as "builds" (i.e. @code{build.yml})
    to @seclink["tasks"]{Tasks} to disambiguate from builds of a job. Jobs have
    builds, and builds are the result of running tasks and resource actions.
  }

  @item{
    In related news, we needed to upgrade the UI to support all of these
    wonderful new flows so we've spruced up the build log page a little. There
    are now individual success/failure markers for each stage and stages that
    are not-interesting (successful) will automatically collapse. There are
    also little icons. Totally rad.
  }

  @item{
    A few of you noticed that having multiple ignore paths in the
    @code{git-resource} wasn't really working properly. Well, we've fixed that.
    We now process the @code{ignore_paths} parameter using @code{.gitignore}
    semantics.
  }
]
