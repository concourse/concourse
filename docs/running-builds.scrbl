#lang scribble/manual

@title[#:tag "running-builds"]{Running Builds}

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
image: docker:///ubuntu#14.04

run:
  path: my-repo/scripts/test
}|

This configuration specifies that the build must run with the
@code{ubuntu:14.04} Docker image, and run the script
@code{my-repo/scripts/test}.

A build's configuration specifies the following:

@defthing[image string]{
  @emph{Required.} The base image of the container. For a
  Docker image, specify in the format @code{docker:///username/repo#tag}
  (rather than @code{username/repo:tag}). If you for whatever reason have an
  extracted rootfs lying around, you can also specify the absolute path to it
  on the worker VM.
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

@defthing[paths {string: string}]{
  @emph{Optional.} This can be specified to configure Concourse
  to place the input resources in custom directories. By default, they are
  placed in directories named after the resource. Go projects, for example,
  may configure something like @code{my-repo: src/github.com/my-org/my-repo},
  as a handy shortcut to put them in a @code{$GOPATH} structure.
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

Once @code{fly} is installed, the only thing to configure is the Glider URL.
Glider is a simple build manager, which just keeps all build state in-memory.
It is designed for use in local Vagrant-deployed instances. When used this way,
no configuration is required; @code{fly} defaults to its port-forwarded address.

For running against a remote deployed instance, simply set @code{$GLIDER_URL}
to its full @code{http://...} address. This pattern is useful for integrating
@code{fly} with existing CI deployments.


@subsection{Using @code{fly}}

The simplest use of @code{fly} is to run it with no arguments in a directory
containing @code{build.yml}:

@codeblock|{
$ cd some-project/
$ fly
}|

This will kick off a build and stream the output back. Fly accepts flags for
pointing it at a specific build config file, and for running with the contents
of a different directory.

Fly will automatically capture @code{SIGINT} and @code{SIGTERM} and abort the
build when received. This makes it easy to use interactively (@code{Ctrl+C}),
and allows it to be worked into other CI systems like Jenkins or GoCD, so that
aborting the build through their UI actually results in aborting the
@code{fly}ing build.


@subsection{Hijacking}

When a build is running, it's off in a container in a VM somewhere.
Traditionally your builds run on your machine, making it easy to see what's
going on with @code{ps auxf}.

Fly preserves this functionality by allowing you to @code{hijack} the build's
container.

When a build is running, simply execute @code{fly hijack} from any terminal.
You will be placed in an interactive session running in the build's container.

@centered{
  @image["images/fly-demo.png"]{Fly Demo}
}

A specific command can also be given, e.g. @code{fly hijack ps auxf} or
@code{fly hijack htop}. This allows for patterns such as @code{watch fly hijack
ps auxf}, which will continuously show the process tree of the current build,
even as the "current build" changes.
