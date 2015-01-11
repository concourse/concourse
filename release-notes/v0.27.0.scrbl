#lang scribble/manual

@title[#:style '(quiet unnumbered)]{v0.27.0}

@itemlist[
  @item{
    A new execution engine has been introduced to the ATC, which replaces the
    Turbine for running builds and checking for resources. This version still
    includes Turbine for backwards-compatibility; the next release will remove
    it.

    This new engine is better at recovering builds that were running; the
    orchestrating ATC can go down while fetching inputs, executing the build,
    or performing outputs, and recover gracefully on start. Caveat: if any of
    these steps *finish* executing while ATC is down, the result will be lost.
    This will be improved in later releases.

    In addition to being a large internal refactor, in the future this new
    engine will also support arbitrary build matrixes, and executing builds
    spanning multiple Garden backends (e.g. Linux and Windows).
  }

  @item{
    A job can now consist solely of inputs and outputs, with no build
    configured. This is very useful for simpler jobs that just transform
    or generate artifacts.
  }

  @item{
    Fixed @code{fly hijack} clobbering the terminal's TTY state.
  }
]
