#lang concourse/docs

@title[#:style '(quiet unnumbered)]{v0.37.0}

@margin-note{
  Run @seclink["fly-sync"]{@code{fly sync}} to upgrade Fly after deploying
  v0.37.0!
}

@itemlist[
  @item{
    @code{fly configure -c} now presents the user with the changes that the new
    configuration applies, and asks for the user to confirm. Committing the
    configurion is done atomically, meaning confirmation always applies what
    you expect. It will reject it if what you've compared against has since
    changed, i.e. another person has run @code{fly configure}.
  }

  @item{
    Concourse is now durable to the worker's Garden servers going down (or not
    being up) in the midst of builds running. This fixes @code{EOF} and
    connection errors causing builds to error.
  }

  @item{
    @code{fly configure -c} is now more helpful when it fails (it actually, you
    know, prints the problem out, rather than silently failing).
  }

  @item{
    The S3 resource now includes @code{url} metadata when fetched as an input.
  }
]
