#lang scribble/manual

@(require "common.rkt")

@title[#:version version]{Concourse: CI that scales with your project}

@image[#:suffixes '(".svg" ".png") "images/example-pipeline"]{Example Pipeline}

Concourse is a CI system composed of simple tools and ideas. It can express
entire pipelines, integrating with arbitrary resources, or it can be used to
execute one-off builds, either locally or in another CI system.

To see a live example, check out
@hyperlink["http://ci.concourse.ci"]{Concourse's own CI pipeline}.

@table-of-contents[]

@include-section{what-and-why.scrbl}
@include-section{concepts.scrbl}
@include-section{getting-started.scrbl}
@include-section{running-builds.scrbl}
@include-section{pipelines.scrbl}
@include-section{implementing-resources.scrbl}
@include-section{release-notes.scrbl}
