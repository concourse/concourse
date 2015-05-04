#lang concourse/docs

@title[#:style '(quiet unnumbered)]{v0.26.0}

@itemlist[
  @item{
    The events protocol has been redone to better support reconnecting. Each
    event is annotated with its own version, and they are versioned
    independently. This fixes @code{fly} hanging after the connection
    times out.
  }

  @item{
    If there are no groups configured, the UI will now show everything,
    rather than nothing.
  }

  @item{
    Lots of internal reworking is under way to make removing the Turbine
    component easier. Events now propagate solely through the database, rather
    than proxying to the Turbine. This is done via PostgreSQL's
    @code{LISTEN}/@code{NOTIFY} feature. Aborting builds is now also propagated
    via database notifications.

    This will enable future build execution engines to be implemented
    without being concerned with clustered ATC implementations.
  }

  @item{
    Fixed the Vagrant Cloud resource to cope with Atlas's API changes.
  }
]
