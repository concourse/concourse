#lang scribble/manual

@(require "../common.rkt")

@title[#:style 'toc #:version version #:tag "task-step"]{@code{task}: execute a task}

Executes a @seclink["tasks"]{Task}, either from a file fetched via the preceding
steps, or with inlined configuration.

If any task in the build plan fails, the build will complete with failure.
By default, any subsequent steps will not be performed. This can be
configured by explicitly setting
@seclink["step-conditions"]{@code{conditions}} on the step after the task.

For example, the following plan fetches a single repository and executes
multiple tasks, using the @seclink["aggregate-step"]{@code{aggregate}} step,
in a build matrix style configuration:

@codeblock|{
plan:
  - get: my-repo
  - aggregate:
      - task: go-1.3
        file: unit.yml
        config:
          params:
            GO_VERSION: 1.3

      - task: go-1.4
        file: ci/go-1.4.yml
        config:
          params:
            GO_VERSION: 1.4
}|

Only if both tasks succeed will the build go green.

@defthing[task string]{
  @emph{Required.} A freeform name for the task that's being executed. Common
  examples would be @code{unit} or @code{integration}.
}

@deftogether[(@defthing[file string] @defthing[config object])]{
  @emph{At least one required.} The configuration for the task's running environment.

  @code{file} points at a @code{.yml} file containing the
  @seclink["configuring-tasks"]{task config}, which allows this to be tracked
  with your resources. The file will provided by the preceding steps, so
  you'll have to know where it ended up by the time you get to this step.

  For example, if the preceding step was the following:

  @codeblock|{
    aggregate:
      - get: something
  }|

  And the @code{something} resource provided a @code{unit.yml} file, you
  would set @code{file: something/unit.yml}. But if the previous step was
  just the single @code{get} step, you would just set @code{file: unit.yml}.

  @code{config} can be defined to inline the task config statically.

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

@inject-analytics[]
