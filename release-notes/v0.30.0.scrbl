#lang scribble/manual

@title[#:style '(quiet unnumbered)]{v0.30.0}

@itemlist[
  @item{
    A breaking change was made to the Cloud Foundry resource. The
    @code{current_app_name} field has been moved from the resource
    configuration to the output configuration.
  }

  @item{
    The Cloud Foundry CLI used by the Cloud Foundry resource has been upgraded
    to v6.9.0.
  }

  @item{
    An even more obvious effect has been chosen for the running build
    indicator. It has the advantage of testing your browser's SVG support at
    the same time.
  }

  @item{
    A build can now run even if it doesn't have any inputs.
  }

  @item{
    Input containers no longer expire if the build container takes too long to
    initialize. This is normally due to large Docker images.
  }
]
