#lang scribble/manual

@title[#:style '(quiet unnumbered)]{v0.29.0}

@itemlist[
  @item{
    Individual resource versions can now be disabled by clicking the toggle
    switch on the resource pages. This is useful if there is a broken or
    pulled input resource version that you want to ignore in automatically
    triggered builds.
  }

  @item{
    The subtle pulsing animation that represented a running build has been
    replaced by a more obvious effect.
  }

  @item{
    If hijack-ing running builds isn't your thing you can now run @code{fly
    intercept} to achieve the same thing.
  }

  @item{
    The S3 resource output metadata now includes the URL of the published file.
    This only applies to public buckets.
  }
]
