#lang scribble/manual

@title[#:style '(quiet unnumbered)]{v0.35.0}

There was a little delay in cutting this release as the laptop^H^H^H^H^H^H high
performance build cluster we use to build the Packer boxes was decomissioned as
we switched over to new hardware.

@itemlist[
  @item{
    Workers can now advertise the platform that they support as well as
    additional tags that can influence the placement of builds. For more
    information see THIS DOCUMENTATION THAT'S TOTALLY WRITTEN. The main
    takeaway is that you'll need to add @code{platform: linux} to all of your
    @code{build.yml}s.
  }

  @item{
    The scheduler semantics have been simplified which has the main effect of
    making sure that disabled resources are not used in manually triggered
    builds.
  }

  @item{
    You can now download a compatible @code{fly} from ATC. Look for the links
    in the bottom right of the main page.
  }

  @item{
    The UI has been spruced up with some little icons. Let us know what you
    think.
  }

  @item{
    We've upgraded Go to version 1.4.2 inside the release. You shouldn't notice
    any difference with Concourse but this should give you a warm fuzzy feeling
    inside.
  }
]
