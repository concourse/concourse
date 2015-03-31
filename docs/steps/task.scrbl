#lang scribble/manual

@(require "../common.rkt")

@title[#:style 'toc #:version version #:tag "task-step"]{@code{task}@aux-elem{: execute a task}}

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
    file: my-repo/unit.yml
    config:
      params:
        GO_VERSION: 1.3

  - task: go-1.4
    file: my-repo/ci/go-1.4.yml
    config:
      params:
        GO_VERSION: 1.4
}|

Only if both tasks succeed will the build go green.

When a task completes, its resulting file tree will be made available as a
source named after the task. This allows subsequent steps to process the
result of a task. For example, the follwing plan pulls down a repo, makes a
commit to it, and pushes the commit to another repo:

@codeblock|{
plan:
- get: my-repo
- task: commit
  file: my-repo/commit.yml
- put: other-repo
  params:
    repository: commit/my-repo
}|

@defthing[task string]{
  @emph{Required.} A freeform name for the task that's being executed. Common
  examples would be @code{unit} or @code{integration}.
}

@deftogether[(@defthing[file string] @defthing[config object])]{
  @emph{At least one required.} The configuration for the task's running
  environment.

  @code{file} points at a @code{.yml} file containing the
  @seclink["configuring-tasks"]{task config}, which allows this to be tracked
  with your resources.

  The first segment in the path should refer to another source from the plan,
  and the rest of the path is relative to that source.

  For example, if in your plan you have the following
  @seclink["get-step"]{@code{get}} step:

  @codeblock|{
    - get: something
  }|

  And the @code{something} resource provided a @code{unit.yml} file, you
  would set @code{file: something/unit.yml}.

  @code{config} can be defined to inline the task config statically.

  If both @code{file} and @code{config} are specified, the value of
  @code{config} is "merged" into the config loaded from the file. This allows
  task parameters to be specified in the task's config file, and overridden in
  the pipeline (i.e. for passing in secret credentials).
}

@defthing[privileged boolean]{
  @emph{Optional. Default @code{false}.} If set to @code{true}, the task will
  run as @code{root} with full capabilities. This is not part of the task
  configuration to prevent privilege escalation via pull requests.

  This is a gaping security hole; use wisely and only if necessary.
}

@inject-analytics[]
