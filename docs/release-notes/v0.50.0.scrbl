#lang concourse/docs

@title[#:style '(quiet unnumbered)]{v0.50.0}

@itemlist[
  @item{
    Viewing a build log is now much more responsive when a lot of output
    appears very quickly, i.e. chatty logs or opening a build in the middle of
    it running.
  }

  @item{
    We have removed support for @code{$ATC_URL} in @code{fly}; use
    @secref{fly-save-target} instead.
  }

  @item{
    We now place artifacts in a random directory, instead of always
    @code{/tmp/build/src}. That was always intended to be a hidden detail, and
    code relying on it should be changed to rely only on @code{$PWD} instead.
  }
]
