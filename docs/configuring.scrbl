#lang scribble/manual

@title{Configuring}

Concourse does not require a wizard to configure. It is designed to be
statically configured via a single file, no larger than it has to be. When
your Concourse deployment burns down, it doesn't matter. Deploy it again
somewhere else, in the exact same configuration.

An entire arbitrarily complicated pipeline can be declared in a single
human-readable config file, and you'll @emph{like it}.

With @hyperlink["https://github.com/cloudfoundry/bosh"]{BOSH}, deploying a
Concourse cluster is easy regardless of the size of the cluster. Scaling up to
handle higher workloads is as trivial as bumping worker instance count.

There are two things to learn how to configure: your individual builds, and your
pipeline, which composes them.

@include-section{configuring/single-builds.scrbl}
@include-section{configuring/pipelines.scrbl}
