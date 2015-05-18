#lang concourse/docs

@(require "common.rkt")

@title[#:style 'toc #:version version #:tag "pipelines"]{Pipelines}

Together, jobs and resources form a pipeline.

Here's an example of a fairly standard unit → integration → deploy pipeline:

@image[#:suffixes '(".svg" ".png") "images/example-pipeline"]{Example Pipeline}

Above, the black boxes are @seclink["resources"]{resources}, and the colored
boxes are @seclink["jobs"]{jobs}, whose color indicates the status of their
most recent @seclink["job-builds"]{build}. It is also possible to
@seclink["configuring-groups"]{group} different sections of a pipeline
together into logical collections.

A pipeline is configured with two sections:
@seclink["configuring-resources"]{@code{resources}} and
@seclink["configuring-jobs"]{@code{jobs}}. For example, the configuration
resulting in the above pipeline is as follows:

@codeblock["yaml"]|{
resources:
  - name: controller
    type: git
    source:
      uri: git@github.com:my-org/controller.git
      branch: master

  - name: worker
    type: git
    source:
      uri: git@github.com:my-org/worker.git
      branch: master

  - name: integration-suite
    type: git
    source:
      uri: git@github.com:my-org/integration-suite.git
      branch: master

  - name: release
    type: git
    source:
      uri: git@github.com:my-org/release.git
      branch: master

  - name: final-release
    type: s3
    source:
      bucket: concourse-releases
      regex: release-(.*).tgz

jobs:
  - name: controller-mysql
    plan:
      - get: controller
      - task: unit
        file: controller/ci/mysql.yml

  - name: controller-postgres
    plan:
      - get: controller
      - task: unit
        file: controller/ci/postgres.yml

  - name: worker
    plan:
      - get: worker
      - task: unit
        file: worker/task.yml

  - name: integration
    plan:
      - aggregate:
          - get: integration-suite
          - get: controller
            passed: [controller-mysql, controller-postgres]
          - get: worker
            passed: [worker]
      - task: integration
        file: integration-suite/task.yml

  - name: deploy
    serial: true
    plan:
      - aggregate:
          - get: release
          - get: controller
            passed: [integration]
          - get: worker
            passed: [integration]
      - task: deploy
        file: release/ci/deploy.yml
      - put: final-release
        params:
          from: deploy/release/build/*.tgz
}|

To learn what the heck that means, read on.

@table-of-contents[]

@include-section{configuring/resources.scrbl}
@include-section{configuring/jobs.scrbl}
@include-section{configuring/groups.scrbl}

@inject-analytics[]
