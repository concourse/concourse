#lang concourse/docs

@title[#:style '(quiet unnumbered)]{v0.34.0}

@itemlist[
  @item{
    Added a GitHub Releases resource. It can pull down and push up multiple
    blobs, and tracks releases via their version. For more details, see its @hyperlink["https://github.com/concourse/github-release-resource/blob/efbfb916836366d729d8172f4f4eadaae34bac45/README.md"]{README}.
  }

  @item{
    The navigation bar is now present on every page.
  }

  @item{
    Upgraded to Consul v0.5. This should fix cases where the workers would lose
    contact with the ATC and never rejoin.
  }

  @item{
    Fixed a panic in the ATC that would happen every time a job with no build
    configuration finished.
  }

  @item{
    Added missing merge strategy binaries to the Git resource; this allows the
    @code{rebase} option to work in more cases.
  }
]
