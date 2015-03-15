#lang scribble/manual

@(require "common.rkt")

@title[#:version version #:tag "running-tasks"]{Tasks}

The smallest configurable unit in a Concourse pipeline is a single task.

Once you have a running Concourse deployment, you can start configuring your
tasks and executing them interactively from your terminal with the
@seclink["fly-cli"]{Fly} commandline tool.

Once you've figured out your tasks's configuration, you can reuse it for a
@seclink["jobs"]{Job} in your @seclink["pipelines"]{Pipeline}.


@section[#:tag "configuring-tasks"]{Configuring a Task}

Conventionally a task's configuration is placed in the root of a repo as
@code{task.yml}. It may look something like:

@codeblock|{
---
platform: linux

image: docker:///ubuntu#14.04

run:
  path: my-repo/scripts/test
}|

This configuration specifies that the task must run with the
@code{ubuntu:14.04} Docker image, and when the task is executed it will run
the script @code{my-repo/scripts/test}.

A task's configuration specifies the following:

@defthing[platform string]{
  @emph{Required.} The platform the task should run on. By convention,
  @code{windows}, @code{linux}, or @code{darwin} are specified. This determines
  the pool of workers that the task can run against. The base deployment
  provides Linux workers.
}

@defthing[tags string]{
  @emph{Optional.} A set of arbitrary tags to determine which workers the task
  should run on. Tags can be omitted from task configuration files and
  overridden by jobs, so that the task config can be reused and run in separate
  environments.
}

@defthing[image string]{
  @emph{Optional.} The base image of the container. For a Docker image, specify
  in the format @code{docker:///username/repo#tag} (rather than
  @code{username/repo:tag}). If you for whatever reason have an extracted
  rootfs lying around, you can also specify the absolute path to it on the
  worker VM.
}

@defthing[inputs [object]]{
  @emph{Optional.} The expected set of inputs for the task.

  If present, the task will validate that its set of dependant inputs are
  present before starting, rather than failing arbitrarily at runtime.

  Each input has the following attributes:

  @nested[#:style 'inset]{
    @defthing[name string]{
      @emph{Required.} The logical name of the input.
    }

    @defthing[path string]{
      @emph{Optional.} The path in the where the input will be placed. If not
      specified, the input's @code{name} is used.
    }
  }
}

@defthing[run object]{
  @emph{Required.} The command to execute in the container.

  @nested[#:style 'inset]{
    @defthing[path string]{
      @emph{Required.} The command to execute, relative to the task's
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
  task via environment variables.

  Use this to provide things like credentials, not to set up the task's
  Bash environment (they do not support interpolation).
}


@section[#:tag "fly"]{Running tasks with @code{fly}}

@seclink["fly-cli"]{Fly} is a command-line tool that can be used to execute
a task configuration against a Concourse deployment. This provides a fast
feedback loop for iterating on the task configuration and your code.

For more information, see @secref{fly-execute}.

@inject-analytics[]
