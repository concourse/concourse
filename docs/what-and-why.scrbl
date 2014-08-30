#lang scribble/manual
 
@title{What & Why}

There are countless CI options out there. Transitioning from one CI system to
another can be a huge investment depending on the size of your project.

Concourse holds the following principles to heart.


@section[#:style 'toc-hidden]{Simple}

Over the years, you've probably learned way too many little details about how
your CI system operates.

Concourse is a response to the complexity introduced by other systems. It is
built on the idea that the best tools can be learned in one sitting.

The focus has not been on adding feature after feature, checkbox after
checkbox. Instead, Concourse defines three primitives that, together, can
express arbitrary features and pipelines.

To learn them, see @secref{Concepts}.


@section[#:style 'toc-hidden]{Usable}

Concourse is optimized for quickly navigating to the pages you most care about.
From the main page, the shortest path from a pipeline view to the console
output of a job's latest failing build is a single click.

From there, the job's entire build history is displayed, and every input for
the job is listed out, with any new inputs highlighted.

The build log is colorized and supports unicode. It emulates your terminal and
gets out of your way.


@section[#:style 'toc-hidden]{Isolated Builds}

Managing the state of the worker VMs in other CI systems is a nightmare. Build
pollution is a constant threat, and the workers have to constantly be tweaked
to make sure they're providing the right things for every build.

In Concourse, the workers are stateless. Every build executes in a container
defined by its own configuration. Multiple teams can use the same Concourse
deployment without worrying about the state of the worker VMs.

See @secref{builds}.


@section[#:style 'toc-hidden]{Scalable, reproducible deployment}

No Concourse deployment is a snowflake. There are no boxes to check; no
configuration happens at runtime.

Concourse is statically configured, and as a result can always be recreated
from scratch with a single BOSH deploy. If your deployment's infrastructure
burns down, just deploy it somewhere else.

As heavier workloads come in, scaling up the workers is as easy as bumping a
number in your BOSH deployment manifest. Scaling back down is the same routine.

See @secref{deploying-with-bosh}.


@section[#:style 'toc-hidden]{Flexible}

Concourse provides the abstractions for you to be able to integrate with
anything you need, and implements most interesting features in terms of this
same primitive.

Most features that other systems implement in the core of the product,
Concourse implements as resources in userland:

@itemlist[
  @item{
    Timed build triggers are implemented as a
    @hyperlink["https://github.com/concourse/time-resource"]{Time} resource.
  }

  @item{
    Integrating with Git repositories is done via the
    @hyperlink["https://github.com/concourse/resource-images/tree/master/git"]{Git} resource.
  }

  @item{
    Auto-delivering stories in @hyperlink["https://pivotaltracker.com"]{Pivotal
    Tracker} is implemented as a
    @hyperlink["https://github.com/concourse/tracker-resource"]{Tracker}
    resource.
  }

  @item{
    Tracking and bumping version numbers is done with the
    @hyperlink["https://github.com/concourse/semver-resource"]{Semver}
    resource. In other systems people end up depending on build numbers, and
    things get funny when they roll over to 1 again in a new deployment.
  }

  @item{
    Integrating with and delivering objects to S3 buckets is provided by the
    @hyperlink["https://github.com/concourse/s3-resource"]{S3} resource.
  }

  @item{
    Building a pipeline for Docker images is provided by the
    @hyperlink["https://github.com/concourse/resource-images/tree/master/docker-image"]{Docker
    Image} resource. Interestingly, this is what is used for delivering the
    resource images themselves.
  }
]

...and more are coming.

By using resources for this, all integration points are explicit parts of the
pipeline, visualized on the front page. It forces the issue of having a
stateless CI system, by externalizing all important artifacts (even version
numbers) to concrete objects outside of the system.

See @secref{resources} and @secref{implementing-resources}.


@section[#:style 'toc-hidden]{Local iteration}

Everyone knows this dance: set up CI, push, build fails. Fix config, push,
build fails... 20 commits later, a green dot and a messy repo history.

Concourse's support for running builds locally eliminates this pesky workflow,
and allows you to trust that your build running locally runs @emph{exactly} the
same way that it runs in your pipeline.

The workflow then becomes: set up CI, configure build locally, @code{fly},
build fails (we can't fix that), fix things up, @code{fly}...

At the end of this, instead of 20 junk commits pushed to your repo, you've
figured out a configuration for both running locally and running in CI.

See @secref{running-builds}.


@section[#:style 'toc-hidden]{Bootstrapped}

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
bunch of simple functions of inputs → outputs.
