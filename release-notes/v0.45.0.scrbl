#lang scribble/manual

@title[#:style '(quiet unnumbered)]{v0.45.0}

Don't worry, this one's backwards-compatible.

If you're reading this and haven't upgraded to v0.44.0 yet, be sure to read
that guy's release notes. It's a doozy.

@itemlist[
  @item{
    One-off builds can now be viewed in the web UI!

    There is an icon at the top right (in the nav bar) that will take you to
    a page listing all builds that have ever run, including one-off builds,
    with the most recent up top.
  }

  @item{
    Resources can now be @emph{paused}, meaning no new versions will be
    collected or used in jobs until it is unpaused. This is useful to cut off
    broken upstream dependencies.
  }

  @item{
    Pipeline configurations can now be parameterized via @code{fly configure}.
    This allows pipeline configurations to be reused, or published with the
    credentials extracted into a separate (private) file.
  }

  @item{
    The @hyperlink["https://github.com/concourse/time-resource"]{Time
    resource} can now be configured to trigger once, or on an interval, within
    a time period. This can be used to e.g. run a build that cleans up
    development environments every night, while no one's at work.
  }

  @item{
    The super verbose and ugly perl warnings while cloning git repositories
    has been fixed!
  }

  @item{
    Some pipeline UI quirks have been fixed. Right-clicking no longer triggers
    dragging around, and the zooming has been bounded (no more losing your
    pipeline!).
  }
]
