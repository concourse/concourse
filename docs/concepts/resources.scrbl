#lang scribble/manual

@title[#:tag "resources"]{Resources}

A resource is any entity that can be checked for new versions, pulled down at a
specific version, and/or pushed up to generate new versions. A common example
would be a git repository, but it can also represent more abstract things like
@hyperlink["https://github.com/concourse/time-resource"]{time}.

Every resource has a @code{type} (for example, @code{git}, or @code{s3}, or
@code{time}). Every resource is also configured with a @code{source}, which
describes where the resource lives (for example, @code|{uri:
git@github.com:foo/bar.git}|).

At its core, Concourse knows nothing about @code{git} or any of these.
Instead, it uses an abstract interface, leaving it to userland to implement all
of them.

This abstraction is immensely powerful, as it does not limit Concourse to
whatever things its authors thought to integrate with. Instead, anyone using
Concourse is free to implement their own resource types, representing whatever
entities they want to integrate with.

Technically, a resource type is implemented by a container image with three
scripts: @code{check} for checking for new versions, @code{in} for pulling it
down, and @code{out} for pushing it up.

Distributing resource types as containers allows them to package their own
dependencies. For example, the Git resource comes with @code{git} installed.

