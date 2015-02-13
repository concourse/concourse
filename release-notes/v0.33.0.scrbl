#lang scribble/manual

@title[#:style '(quiet unnumbered)]{v0.33.0}

@itemlist[
  @item{
    Now works with the latest Garden Linux again.
  }

  @item{
    Reworked the build view's ansi parsing, which now uses an external library.
    This fixes some rendering issues (e.g. long swaths of text in a single color
    now preserve the color for the whole region).
  }
]
