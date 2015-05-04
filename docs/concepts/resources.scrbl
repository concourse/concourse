#lang concourse/docs

@(require "../common.rkt")

@title[#:version version #:tag "resources"]{Resources}

A resource is any entity that can be checked for new versions, pulled down
at a specific version, and/or pushed up to idempotently create new versions.
A common example would be a git repository, but it can also represent more
abstract things like
@hyperlink["https://github.com/concourse/time-resource"]{time itself}.

At its core, Concourse knows nothing about things like @code{git}. Instead,
it consumes a generic interface implemented by @emph{resource types}. This
allows Concourse to be extended by configuring workers with resource type
implementations.

This abstraction is immensely powerful, as it does not limit Concourse to
whatever things its authors thought to integrate with. Instead, as a user of
Concourse you can just reuse resource type implementations, or
@seclink["implementing-resources"]{implement your own}.

@inject-analytics[]
