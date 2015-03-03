#lang scribble/manual

@(require "common.rkt")

@title[#:version version #:tag "fly-cli"]{The Fly CLI}

The @code{fly} tool is a command line interface to Concourse. It is used for
a number of tasks from connecting to a shell in one of your build's
containers to uploading new pipeline configuration into a running Concourse.
Learning how to use @code{fly} will make using Concourse faster and more
useful.

You can download @code{fly} from an Concourse. There are download links for
common platforms in the bottom right hand corner of the main page.

@section{Targeting your Concourse}

Fly works with an already deployed Concourse. If you don't already have one of
these you should follow the @seclink["deploying-with-vagrant"]{Deploying
with Vagrant} or @seclink["deploying-with-bosh"]{Deploying with BOSH} guides
to deploy a Concourse.

Once, you've deployed an Concourse you can tell @code{fly} to target it in a
couple of ways. You can either set the environment variable @code{ATC_URL}
or you can give the command line option @code{--atcURL}. For example, if we
wanted to run @code{fly sync} (don't worry what this means just yet) while
pointing at an Concourse that we normally reach by going to
@code{http://ci.example.com} then you could run either of the following:

@codeblock|{
$ fly --atcURL 'http://ci.example.com' sync

$ ATC_URL='http://ci.example.com' fly sync
}|

@margin-note{
  The default Vagrant address is set as the default in @code{fly}. This means
  that you don't need to do anything extra if you are using the Vagrant boxes
  to deploy Concourse.
}

The single quotes aren't always required but if you need to put HTTP basic
authentication credentials inline then they can help by avoiding the need to
escape special characters in passwords. For example:

@codeblock|{
$ fly --atcURL 'http://username:p@$$w0rd@ci.example.com' sync
}|

If your Concourse uses SSL but does not have a CA signed certificate then
you can use the @code{-k} or @code{--insecure} flag in order to make fly not
check the remote certificates.

For the rest of this document it is assumed you are setting the target in
each of the commands and so it will not be included for brevity.

@section{@code{execute}: Submitting Local Builds}

One of the most common use cases of @code{fly} is taking a local project on
your computer and submitting it up with a build configuration to be run
inside a container in Concourse. This is useful to build Linux projects on
OS X or to avoid all of those debugging commits when something is configured
differently between your local and remote setup.

If you have a build configuration called @code{build.yml} that describes a
build that only requires the files in the current directory (e.g. most unit
tests and simple integration tests) then you can just run:

@codeblock|{
$ fly execute
}|

And your files will be uploaded and the build will be executed with them.

If your build configuration is in a non-standard location then you can
specify it using the @code{-c} or @code{--config} argument like so:

@codeblock|{
$ fly execute -c tests.yml
}|

If you have many extra files or large files in your currect directory that
would normally be ignored by your version control system then you can use
the @code{-x} or @code{--exclude-ignored} flags in order to limit the files
that you send to Concourse to just those that are not ignored.

If your build needs to run as @code{root} then you can specify the @code{-p}
or @code{--privileged} flag.

The default @code{fly} command is @code{execute} and so you can omit and
just run the following to get the same effect:

@codeblock|{
$ fly
}|

@subsection{Multiple Inputs to Locally Submitted Builds}

Builds in Concourse can take multiple inputs. Up until now we've just been
submitting a single input (our current working directory) that has the same
name as the directory.

Builds can specify the inputs that they require (for more information, refer
to the @seclink["configuring-builds"]{configuring builds} documentation).
For @code{fly} to upload these inputs you can use the @code{-i} or
@code{--input} arguments with name and path pairs. For example:

@codeblock|{
$ fly execute -i code=. -i stemcells=../stemcells
}|

This would work together with a build.yml if its @code{inputs:} section was
as follows:

@codeblock|{
inputs:
  - name: code
  - name: stemcells
}|

If you specify an input then the default input will no longer be added
automatically and you will need to explicitly list it (the code input
above).

This feature can be used to mimic other resources and try out combinations
of input that would normally not be possible in a pipeline.

@section{@code{configure}: Reconfiguring Concourse}

Fly can be used to fetch and update the current pipeline configuration
inside an Concourse. This is achieved by using the @code{configure} command.
For example, to fetch the current configuration of an Concourse and print it
on @code{STDOUT} run the following:

@codeblock|{
$ fly configure
}|

To get JSON instead of YAML you can use the @code{-j} or @code{--json} argument.

To submit configuration to Concourse from a file on your local disk you can
use the @code{-c} or @code{--config} flag, like so:

@codeblock|{
$ fly configure --config pipeline.yml
}|

This will present a diff of the changes and ask you to confirm the changes.
If you accept then Concourse's pipeline configuration instantly to the
pipeline definition in the YAML file specified.

@section{@code{intercept}: Accessing a running or recent build}

Sometimes it's helpful to be on the same machine as your builds so that you
can profile or inspect them as they run or see the state the machine at the
end of a run. Due to Concourse running builds in containers on remote
machines this would typically be hard to access. To this end, there is a
@code{fly intercept} command that will give you a shell inside the most
recent one-off build that was submitted to Concourse. For example, running
the following will run a build and then enter the finished build's
container:

@margin-note{
  Be warned, if more than one person is using a Concourse server for running
  one-off builds then you may end up in a build that you did not expect!
}

@codeblock|{
$ fly
$ fly intercept
}|

Containers are around for a short time after a build in order to allow
people to intercept them.

You can also intercept builds that were run in your pipeline. By using the
@code{-j} or @code{--job} and @code{-b} or @code{--build} you can pick out a
specific job and build to intercept.

The @code{-p} or @code{--privileged} flag is used to create a shell in a
remote container that is running as the @code{root} user.

@margin-note{
  The command @code{fly hijack} is an alias of @code{fly intercept}. Both can
  be used interchangably.
}

@centered{
  @image["images/fly-demo.png"]{Fly Demo}
}

A specific command can also be given, e.g. @code{fly intercept ps auxf} or
@code{fly intercept htop}. This allows for patterns such as @code{watch fly
intercept ps auxf}, which will continuously show the process tree of the
current build, even as the "current build" changes.

@section{@code{sync}: Update your local copy of @code{fly}}

Occasionally we add additional features to @code{fly} or make changes to the
communiction between it and Concourse. To make sure you're running the
latest and greatest version that works with the Concourse you are targeting
we provide a command called @code{sync} that will update your local
@code{fly}. It can be used like so:

@codeblock|{
$ fly sync
}|

@section{@code{watch}: View logs of in-progress builds}

Concourse emits streaming colored logs on the website but it can be helpful
to have the logs availiable to the command line. (e.g. so that they can be
processed by other commands).

The @code{watch} command can be used to do just this. You can also view
builds that were run in your pipeline. By using the @code{-j} or
@code{--job} and @code{-b} or @code{--build} you can pick out a specific job
and build to watch. For example, the following command will either show the
archived logs for an old build if it has finished running or it will stream
the current logs if the build is still in progress.

@codeblock|{
$ fly watch --job tests --build 52
}|
