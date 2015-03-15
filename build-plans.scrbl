#lang scribble/manual

@(require "common.rkt")

@title[#:style 'toc #:version version #:tag "build-plans"]{Build Plans}

Each @seclink["jobs"]{Job} has a single build plan. When a build of a job is
created, the plan determines what happens: @seclink["resources"]{Resources}
may be fetched or updated, and @seclink["tasks"]{Tasks} may be performed.

A new build of the job is scheduled whenever any of the resources described
by the first @seclink["get-step"]{@code{get}} steps have new versions.

To visualize the job in the pipeline, resources that appear as @code{get}
steps are drawn as inputs, and resources that appear in @code{put} steps
appear as outputs.

A simple unit test job may look something like:

@codeblock|{
name: banana-unit
plan:
  - get: banana
  - task: unit
    file: task.yml
}|

This job says: @seclink["get-step"]{@code{get}} the @code{my-project}
resource, and run a @seclink["task-step"]{@code{task}} step called
@code{unit}, using the configuration from @code{task.yml}.

When new versions of @code{banana} are detected, a new build of
@code{banana-unit} will be scheduled.

Jobs can depend on resources that are produced by or pass through upstream
jobs, by configuring @code{passed: [job-a, job-b]} on the
@seclink["get-step"]{@code{get}} step.

Putting these pieces together, if we were to propagate @code{banana} from
the above example into an integration suite with another @code{apple}
component (pretending we also defined its @code{apple-unit} job), the
configuration for the integration job may look something like:

@codeblock|{
name: fruit-basket-integration
plan:
  - aggregate:
      - get: banana
        passed: [banana-unit]
      - get: apple
        passed: [apple-unit]
      - get: integration-suite
  - task: integration
    file: integration-suite/task.yml
}|

Note the use of the @seclink["aggregate-step"]{@code{aggregate}} step to
collect multiple inputs at once.

With this example we've configured a tiny pipeline that will automatically
run unit tests for two components, and continuously run integration tests
against whichever versions pass both unit tests.

This can be further chained into later stages; for example, you may want to
continuously deliver an artifact built from whichever components pass
@code{fruit-basket-integration}.

To push artifacts, you would use a @seclink["put-step"]{@code{put}} step
that targets the destination resource. For example:

@codeblock|{
name: deliver-food
plan:
  - aggregate:
      - get: banana
        passed: [fruit-basket-integration]
      - get: apple
        passed: [fruit-basket-integration]
      - get: baggy
  - task: shrink-wrap
    file: baggy/shrink-wrap.yml
  - put: bagged-food
    params:
      bag: baggy/bagged.tgz
}|

This presumes that there's a @code{bagged-food}
@seclink["resources"]{resource} defined, which understands that the
@code{bag} parameter points to a file to ship up to the resource's location.

Note that both @code{banana} and @code{apple} list the same job as an
upstream dependency. This guarantees that @code{deliver-food} will only
trigger when a version of both of these dependencies pass through the same
build of the integration job (and transitively, their individual unit jobs).
This prevents bad apples or bruised bananas from being delivered.

For a reference on each type of step, read on.

@table-of-contents[]

@section[#:tag "get-step"]{@code{get}: fetch a resource}

Fetches a resource, making it available to subsequent steps via the given name.

@nested[#:style 'inset]{
  @defthing[get string]{
    @emph{Required.} The logical name of the resource being fetched. This name
    satisfies logical inputs to a @seclink["tasks"]{Task}, and may be referenced
    within the plan itself (e.g. in the @code{file} attribute of a
    @seclink["task-step"]{@code{task}} step).
  }

  @defthing[resource string]{
    @emph{Optional. Defaults to @code{name}.} The resource to fetch, as
    configured in @seclink["configuring-resources"]{@code{resources}}.
  }

  @defthing[passed [string]]{
    @emph{Optional.} When configured, only the versions of the resource that
    appear as outputs of the given list of jobs will be considered when
    triggering and fetching.

    Note that if multiple @code{get}s are configured with @code{passed}
    constraints, all of the mentioned jobs are correlated. That is, with the
    following set of inputs:

    @codeblock|{
    plan:
    - get: a
      passed: [a-unit, integration]
    - get: b
      passed: [b-unit, integration]
    - get: x
      passed: [integration]
    }|

    This means "give me the versions of @code{a}, @code{b}, and @code{x} that
    have passed the @emph{same build} of @code{integration}, with the same
    version of @code{a} passing @code{a-unit} and the same version of
    @code{b} passing @code{b-unit}."

    This is crucial to being able to implement safe "fan-in" semantics as
    things progress through a pipeline.
  }

  @defthing[params object]{
    @emph{Optional.} A map of arbitrary configuration to forward to the
    resource's @code{in} script.
  }

  @defthing[trigger boolean]{
    @emph{Optional. Default @code{true}.} Normally, when any of a job's
    dependent resources have new versions, a new build of the job is triggered.

    Setting this to @code{false} effectively makes it so that if the only
    changed resource is this one (or other resources with
    @code{trigger: false}), the job should not trigger.

    The job can still be manually triggered.
  }
}


@section[#:tag "put-step"]{@code{put}: update a resource}

Pushes to the given @seclink["resources"]{Resource} using the state from the
preceding steps, if available.

@nested[#:style 'inset]{
  @defthing[put string]{
    @emph{Required.} The name of the resource.
  }

  @defthing[resource string]{
    @emph{Optional. Defaults to @code{name}.} The resource to update,
    as configured in @seclink["configuring-resources"]{@code{resources}}.
  }

  @defthing[params object]{
    @emph{Optional.} A map of arbitrary configuration to forward to the
    resource's @code{out} script.
  }
}

For example, the following plan would fetch a repo and push it to another repo:

@codeblock|{
plan:
  - get: repo-a
  - put: repo-b
    params:
      repository: ./
}|


@section[#:tag "task-step"]{@code{task}: run a task}

Executes a @seclink["tasks"]{task}, either from a file fetched via a
@seclink["get-step"]{@code{get}} step, or by inlined configuration.

If any task in the build plan fails, the build will complete with failure.
By default, any subsequent steps will not be performed. This can be
configured by explicitly setting
@seclink["step-conditions"]{@code{conditions}} on the step after the task.

@nested[#:style 'inset]{
  @defthing[task string]{
    @emph{Required.} A freeform name for the task that's being executed. Common
    examples would be @code{unit} or @code{integration}.
  }

  @deftogether[(@defthing[file string] @defthing[config object])]{
    @emph{Required.} The configuration for the task's running environment.

    @code{file} points at a @code{.yml} file containing the
    @seclink["configuring-tasks"]{task config}, which allows this to be tracked
    with your resources. The file is provided by an fetched resource, so
    typically this value may be @code{resource-name/task.yml}.

    @code{config} can be defined to inline the same configuration as
    @code{task.yml}.

    If both are specified, the value of @code{config} is "merged" into the
    config loaded from the file. This allows task parameters to be specified
    in the task's config file, and overridden in the pipeline (i.e. for passing
    in secret credentials).
  }

  @defthing[privileged boolean]{
    @emph{Optional. Default @code{false}.} If set to @code{true}, the task will
    run as @code{root} with full capabilities. This is not part of the task
    configuration to prevent privilege escalation via pull requests.

    This is a gaping security hole; use wisely and only if necessary.
  }
}


@section[#:tag "aggregate-step"]{@code{aggregate}: run steps in parallel}

Performs all of the given steps in parallel, aggregating their resulting
file tree for subsequent steps.

@nested[#:style 'inset]{
  @defthing[aggregate [step]]{
    The steps to execute in parallel.
  }
}

For example, if you have a @seclink["tasks"]{Task} that expects multiple inputs,
you would precede it with an @code{aggregate} containing a
@seclink["get-step"]{@code{get}} step for each input:

@codeblock|{
plan:
  - aggregate:
      - get: component-a
      - get: component-b
      - get: integration-suite
  - task: integration
    file: integration-suite/task.yml
}|


@section[#:tag "do-step"]{@code{do}: run steps in series}

Simply performs the given steps serially, with the same semantics as if they
were at the top level step listing.

@nested[#:style 'inset]{
  @defthing[do [step]]{
    The steps to execute in series.
  }
}

This can be used to perform multiple steps serially in the branch of an
@seclink["aggregate-step"]{@code{aggregate}} step:

@codeblock|{
plan:
  - aggregate:
      - task: unit
      - do:
          - get: something-else
          - task: something-else-unit
}|


@section[#:tag "step-conditions"]{@code{conditions}: conditionally perform a step}

Any of the above steps can be made conditional by adding the @code{conditions}
attribute to it.

If the condition does not match, execution of the sequence of steps does not
continue even after the conditional step. If you have mulitple conditions
you'd like to branch off of, you can wrap it in an
@seclink["aggregate-step"]{@code{aggregate}} step:

@nested[#:style 'inset]{
  @defthing[conditions [string]]{
    @emph{Optional. Default @code{[success]}.} The conditions under which to
    perform this step. For example, @code{[failure]} causes the step to be
    performed if the previous step's task(s) fail, while
    @code{[success, failure]} means the step will be performed regardless of
    the result.
  }
}

For example, the following will perform the second task only if the previous
one fails:

@codeblock|{
plan:
  - task: unit
    file: unit.yml
  - conditions: [failure]
    task: alert
    file: alert.yml
}|

If the condition does not match, the step, and all subsequent steps, are not
executed.

If you have mulitple conditions you'd like to branch off of, you can wrap
two conditional steps in an @seclink["aggregate-step"]{@code{aggregate}
step}:

@codeblock|{
plan:
  - task: unit
    file: unit.yml
  - aggregate:
      - conditions: [success]
        task: update-status
        file: update-status.yml
        params:
          status: good
      - conditions: [failure]
        task: update-status
        file: update-status.yml
        params:
          status: bad
}|

@inject-analytics[]
