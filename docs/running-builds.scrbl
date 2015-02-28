#lang scribble/manual

@(require "common.rkt")

@title[#:version version #:tag "running-builds"]{Running Builds}

The smallest configurable unit in Concourse is a single build.

Once you have a running Concourse deployment, you can start configuring your
builds and executing them interactively from your terminal with the
@seclink["fly"]{Fly} commandline tool.

Once you've figured out your build's configuration, you can reuse it for a
@seclink["jobs"]{Job} in your @seclink["pipelines"]{Pipeline}.


@section[#:tag "configuring-builds"]{Configuring a Build}

Conventionally a build's configuration is placed in the root of a repo as
@code{build.yml}. It may look something like:

@codeblock|{
---
platform: linux

image: docker:///ubuntu#14.04

run:
  path: my-repo/scripts/test
}|

This configuration specifies that the build must run with the
@code{ubuntu:14.04} Docker image, and run the script
@code{my-repo/scripts/test}.

A build's configuration specifies the following:

@defthing[platform string]{
  @emph{Required.} The platform the build should run on. By convention,
  @code{windows}, @code{linux}, or @code{darwin} are specified. This determines
  the pool of workers that the build can run against. The base deployment
  provides Linux workers.
}

@defthing[tags string]{
  @emph{Optional.} A set of arbitrary tags to determine which workers the build
  should run on. This is typically left empty for builds configs, and overridden
  by jobs. This is to keep build configs limited to actual requirements, rather
  than worker placement/pipeline config.
}

@defthing[image string]{
  @emph{Optional.} The base image of the container. For a Docker image, specify
  in the format @code{docker:///username/repo#tag} (rather than
  @code{username/repo:tag}). If you for whatever reason have an extracted
  rootfs lying around, you can also specify the absolute path to it on the
  worker VM.
}

@defthing[inputs [object]]{
  @emph{Optional.} The expected set of inputs for the build.

  If present, the build will validate that its set of dependant inputs are
  present before starting, rather than failing arbitrarily at runtime.

  Each input has the following attributes:

  @nested[#:style 'inset]{
    @defthing[name string]{
      @emph{Required.} The logical name of the input.
    }

    @defthing[path string]{
      @emph{Optional.} The path in the build where the input will be placed. If
      not specified, the input's @code{name} is used.
    }
  }
}

@defthing[run object]{
  @emph{Required.} The command to execute in the container.

  @nested[#:style 'inset]{
    @defthing[path string]{
      @emph{Required.} The command to execute, relative to the build's
      working directory. For a script living in a resource's repo, you must
      specify the full path to the resource, i.e.
      @code{my-resource/scripts/test}.
    }

    @defthing[args [string]]{
      @emph{Optional.} Arguments to pass to the command. Note that when
      executed with Fly, any arguments passed to Fly are appended to this
      array.
    }
  }

  Note that this is @emph{not} provided as a script blob, but explicit
  @code{path} and @code{args} values; this allows @code{fly} to forward
  arguments to the script, and forces your config @code{.yml} to stay fairly
  small.
}

@defthing[params {string: string}]{
  @emph{Optional.} A key-value mapping of values that are exposed to the
  build via environment variables.

  Use this to provide things like credentials, not to set up the build's
  Bash environment (they are not interpolated).
}


@section[#:tag "fly"]{Running with @code{fly}}

@hyperlink["https://github.com/concourse/fly"]{Fly} is a command-line tool that
executes a build configuration against a Concourse deployment.

Typically this is used in combination with a
@seclink["deploying-with-vagrant"]{Vagrant}-deployed VM, to provide fast local
feedback on a build, which executes exactly the same way that it would in a
pipeline.


@subsection{Installation}

Currently, Fly must be built manually as there are no prebuilt releases. If you
have Go installed, this is as easy as:

@codeblock|{
go get github.com/concourse/fly
}|


@subsection{Using @code{fly}}

@margin-note{
  Flying with a remote Concourse can be done by setting @code{$ATC_URL} to its
  full @code{http://...} address.
}

Once @code{fly} is installed, the only thing to configure is the ATC URL.

If you've set up a local @seclink["deploying-with-vagrant"]{Vagrant-deployed}
instance, no further configuration is necessary: @code{fly} defaults to looking
at the ports forwarded through this configuration.

Otherwise, to execute @code{fly} against an arbitrary deployment, just set
@code{$ATC_URL} to the URL of the ATC in your deployment (e.g.
@code|{https://user:pass@ci.myproject.com:443}|).

The simplest use of @code{fly} is to run it with no arguments in a directory
containing @code{build.yml}:

@codeblock|{
$ cd some-project/
$ fly
}|

This will kick off a build and stream the output back. Fly accepts flags for
pointing it at a specific build config file, and for running with the contents
of a different directory. Consult @code{fly --help} to see all of the flags.

Fly will automatically capture @code{SIGINT} and @code{SIGTERM} and abort the
build when received. Generally Fly tries to be as thin of a proxy as possible;
this allows it to be transparently composed with other toolchains.

For more information on how to use @code{fly}, see @@seclink["fly-cli"]{the
Fly CLI} section.
