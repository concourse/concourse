#lang concourse/docs

@(require "../common.rkt")

@title[#:style 'toc #:version version #:tag "configuring-groups"]{@code{groups}@aux-elem{: Splitting up your pipeline into sections}}

A pipeline may optionally contain a section called @code{groups}. As more
resources and jobs are added to a pipeline it can become difficult to navigate.
Pipeline groups allow you to group jobs together under a header and have them
show on different tabs in the user interface. Groups have no functional effect
on your pipeline.

A simple grouping for the pipeline above may look like:

@codeblock["yaml"]|{
groups:
- name: tests
  jobs:
  - controller-mysql
  - controller-postgres
  - worker
  - integration
- name: deploy
  jobs:
  - deploy
}|

This would display two tabs at the top of the home page: "tests" and "deploy".

For a real world example of how groups can be used to simplify navigation and
provide logical grouping, see the groups used at the top of the page in the
@hyperlink["https://ci.concourse.ci"]{Concourse pipeline}.

Each configured group consists of the following attributes:

@defthing[name string]{
  @emph{Required.} The name of the group. This should be short and simple as
  it will be used as the tab name for navigation.
}

@defthing[jobs [string]]{
  @emph{Optional.} A list of jobs that should appear in this group. A job may
  appear in multiple groups. Neighbours of jobs in the current group will also
  appear on the same page in order to give context of the location of the
  group in the pipeline.
}

@defthing[resources [string]]{
  @emph{Optional.} A list of resources that should appear in this group.
  Resources that are inputs or outputs of jobs in the group are automatically
  added; they do not have to be explicitly listed here.
}

@inject-analytics[]
