#lang concourse/docs

@title[#:style '(quiet unnumbered)]{v0.25.0}

@itemlist[
  @item{
    The pipeline UI has been completely redone. The previous UI was too prone
    to dramatic visual changes as config changed, and encouraged 'designing'
    the pipeline via config.

    This new UI more truthfully represents the propagation of resources through
    the pipeline. Rather than singular labeled arrows between jobs, jobs now
    have entrance and exit lines, one per resource, and the resources "thread
    through" the UI. Hovering over a resource or a line will highlight all
    occurrences of that resource throughout the pipeline.

    It's much easier to quickly collect information with the new UI. For
    example, the taller a job's box is, the more resources it affects. You can
    also typically look one column to the left of a job to know exactly where
    its inputs are coming from.
  }

  @item{
    Fly now has a @code{watch} subcommand for streaming a build's output.
  }

  @item{
    The API now respects the same viewability rules as the web UI with regard
    to authentication. This makes it easier/safer to point third-party build
    monitoring tools at your CI.
  }

  @item{
    Jobs with a running build are now shown as the same color as their previous
    state (i.e. green or red). This makes it easier to tell if the pipeline is
    blocked - rather than looking at a monitor and seeing yellow, you'll now
    see if it's trying to fix a broken build, and know not to push.
  }

  @item{
    A @hyperlink["https://github.com/concourse/cf-resource"]{Cloud Foundry
    Resource} has been added, supporting @code{cf push} with blue-green
    deploys.
  }
]
