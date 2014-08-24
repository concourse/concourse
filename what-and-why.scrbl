#lang scribble/manual
 
@title{What & Why}

There are countless CI options out there. Transitioning from one CI system to
another can be a huge investment depending on the size of your project.

So, why should you care about Concourse?


@section{Easy to learn}

Over the years, you've probably learned way too many little details about how
your CI system operates.

Concourse is a response to the complexity introduced by other systems. It is
built from as few distinct concepts as possible. To learn them, see
@secref{Concepts}.


@section{No build pollution}

Every build executes in a container, isolated from the rest. Multiple teams can
use the same Concourse deployment without worrying about stepping on each
other's toes (barring resource contention).

The base image for each job's builds is configurable, and supports Docker
images. Control your build's runtime environment as part of your project that
needs it, not your worker VMs.


@section{Scalable, reproducible deployment}

Concourse is statically configurable (by humans) and can be (re)created from
scratch from a single BOSH deploy. Adding more workers is as easy as bumping a
number in your BOSH deployment manifest.

See @secref{Configuring} and @secref{deploying-with-bosh}.


@section{Usable}

Concourse is optimized for quickly navigating to the pages you most care about.
From the main page, the shortest path from a pipeline view to the console
output of a job's latest failing build is a single click.

From there, the job's entire build history is displayed, and every input for
the job is listed out, with any new inputs highlighted.

The build log is colorized and supports unicode. It emulates your terminal and
gets out of your way.


@section{Flexible}

Concourse provides the abstractions for you to be able to integrate with
anything you need, and implements most interesting features in terms of this
same primitive.

See @secref{Resources} and @secref{implementing-resources}.


@section{Local iteration}

Everyone knows this dance: set up CI, push, build fails. Fix config, push,
build fails... 20 commits later, success.

Concourse's support for running builds locally eliminates this pesky workflow,
and allows you to trust that your build running locally runs @emph{exactly} the
same way that it runs in your CI system.

The workflow then becomes: set up CI, configure build locally, @code{fly},
build fails (we can't fix that), fix things up, @code{fly}...

At the end of this, instead of 20 junk commits pushed to your repo, you've
figured out a configuration for both running locally and running in CI.


@section{Bootstrapped}

Proving all of this works is hard without having a real use case. Thankfully,
Concourse itself is a sufficiently large piece of work that its own pipeline
has been plenty to cut its teeth on.

@centered{
  @hyperlink["concourse-pipeline.png"]{
    @image[#:scale 0.3 "images/concourse-pipeline.png"]{Concourse Pipeline}
  }
}

Initially this array of squares may be a lot to take in, but on your own
projects, where @emph{reality} is this complicated, you'll appreciate the
straightforward expression of every relationship.

At the start of the pipeline are jobs configured for each individual component.
These jobs simply run their unit tests, and are the first line of defense.

The versions of each component that make it through this stage are then fed
into an integration job, which spins every component up in a room and makes
them talk to each other.

From there, the Docker images used for the resource types within the
integration build are shipped, and the ref of each successful resource is
bumped in the BOSH release repository.

Because the release repo changed, a Deploy job kicks in, which literally
@emph{deploys to the same instance running the Deploy job}. Concourse's own
pipeline drives out the need for deploys to not trash every running build.

After a deploy succeeds, the Concourse version number resource is bumped, and
new artifacts are available for shipping into a new release.

At any point in time, I can walk in and trigger the @code{shipit} job, which
takes the most recently built release candidate, bumps its version resource to
a final number (@code{0.3.0.rc.3} → @code{0.3.0}), and uploads a @code{.tgz}
to the S3 bucket containing final releases.

Though the above chain of events may sound complicated, in reality it is just a
bunch of simple functions of inputs &rarr; outputs.
