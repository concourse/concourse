#lang scribble/manual

@title[#:style '(quiet unnumbered)]{v0.31.0}

@itemlist[
  @item{
    Fixed a bug that caused containers not to be released after builds
    completed. This caused defunct containers to build up on the box and give
    Garden a run for its money.
  }

  @item{
    We're tracking Garden master again. It's a good day.
  }

  @item{
    The Docker image resource would fail to download images over 1G in size.
    Since some people decide to put way too much junk in their images we're
    increasing this to 10G. Don't make us do it again.

    Busybox, people. Busybox.
  }

  @item{
    Outputs which didn't have any metadata no longer visually hang forever. Now
    you can see that sweet Pivotal Tracker resource output again.
  }
]
