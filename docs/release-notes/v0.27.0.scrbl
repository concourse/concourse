#lang concourse/docs

@title[#:style '(quiet unnumbered)]{v0.27.0}

Large internal restructuring and refactoring geared towards removing the Turbine
component.

A few things in the deployment manifest have changed as a result. See the @hyperlink["https://github.com/concourse/concourse/tree/102398a7e4701dec0fe8cdfaf99c415b64542e12/manifests"]{example manifests}
to see what's new.

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

  @item{
    Garden Workers can be dynamically registered and listed through the API.
    Containers are distributed among them randomly.

    Later this will be expanded to support workers of various platforms, and
    various supported resource types.
  }
]
