#lang scribble/manual

@title[#:style '(quiet unnumbered)]{v0.38.0}

@margin-note{
  Run @seclink["fly-sync"]{@code{fly sync}} to upgrade Fly after deploying
  v0.38.0!
}

@itemlist[
  @item{
    Extra keys are now detected during config updates and rejected if present.
    This should catch common user errors (e.g. forgetting to nest resource
    config under @code{source}) and backwards-incompatible changes more safely
    (e.g. renaming @code{trigger} to something else).
  }

  @item{
    The GitHub Release resource now @emph{actually} supports being used as an
    input. We had previously forgotten to actually wire in the command. Woops.
  }
]
