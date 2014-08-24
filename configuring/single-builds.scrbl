#lang scribble/manual

@title{Single Builds}

The smallest configurable unit in Concourse is a single build.

Conventionally a build's configuration is placed in the root of a repo as
@code{build.yml}. It may look something like:

@codeblock|{
image: docker:///ubuntu#14.04
run:
  path: my-repo/scripts/test
}|

This configuration specifies that the build must run with the
@code{ubuntu:14.04} Docker image, and run the script
@code{my-repo/scripts/test}.

Builds can be executed locally with the [Fly](/components/fly) commandline tool.
This enables you to run builds on your development machine exactly the same way
your CI runs it (assuming your CI points at the same build @code{.yml} config).

If you have an existing CI deployment, you can use Fly in combination with it,
to at least get the containerization and local development features without a
drastic change to your CI infrastructure.

A build's configuration specifies the following:

@itemlist[
  @item{
    @code{image}: @emph{Required.} The base image of the container. For a
    Docker image, specify in the format @code{docker:///username/repo#tag}
    (rather than @code{username/repo:tag}). If you for whatever reason have an
    extracted rootfs lying around, you can also specify the absolute path to it
    on the worker VM.
  }

  @item{
    @code{run}: @emph{Required.} The path to a script to execute, and any
    arguments to pass to it. Note that this is @emph{not} provided as a script
    blob, but explicit @code{path} and @code{args} values; this allows
    @code{fly} to forward arguments to the script, and forces your config
    @code{.yml} to stay fairly small.
  }

  @item{
    @code{params}: @emph{Optional.} A key-value mapping of values that are
    exposed to the build via environment variables. Use this to provide things
    like credentials, not to set up the build's environment (they are not
    interpolated).
  }

  @item{
    @code{paths}: @emph{Optional.} This can be specified to configure Concourse
    to place the input resources in custom directories. By default, they are
    placed in directories named after the resource. Go projects, for example,
    may configure something like @code{my-repo: src/github.com/my-org/my-repo},
    as a handy shortcut to put them in a @code{$GOPATH} structure.
  }
]
