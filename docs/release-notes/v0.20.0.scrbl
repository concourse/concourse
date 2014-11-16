#lang scribble/manual

@title[#:style '(quiet unnumbered)]{v0.20.0}

Backwards-compatible, with more high-availability progress, and now
reconfigurable via the API.

This also marks the first release available as a pre-built Vagrant box. For
a fast local development bootstrap, execute the following anywhere (assuming
Vagrant is installed):

@codeblock|{
vagrant init concourse/lite
vagrant up
export ATC_URL=http://192.168.100.4:8080
}|

And then @code{fly} away!

Full release notes:

@itemlist[
  @item{
    The pipeline can now be reconfigured via ATC's API. Existing deployments
    will remain working, but cannot be configured through their API, as their
    source of truth is still the BOSH deployment manifest.

    To switch to being configured via the API, first create a single @code{.yml}
    file containing your current configuration. This can be done by extracting
    the @code{pipeline} config from your existing manifest, or by running
    @code{fly configure} with no flags.

    Once you have your pipeline as a standalone config file, apply it by running
    @code{fly configure -c <path/to/pipeline.yml}. To reconfigure, edit the file
    and run the same command.
  }

  @item{
    The ATC's locking for performing tasks like scheduling builds, checking for
    resources, and tracking build progress is now fine-grained. This means it's
    more likely for the workload to spread across a cluster of ATCs, but the
    distribution is still nowhere near even.
  }

  @item{
    The Git resource now supports blacklisting/whitelisting files, and
    configuring which submodules get synced. These features are a big help when
    there's one big repository being threaded through a pipeline for different
    purposes at different stages.

    For more information, see the
    @hyperlink["https://github.com/concourse/git-resource/blob/46c88f4556dc3d4f3d602dbec34b3aa69fa41c86/README.md"]{Git Resource's README.md}.
  }
]
