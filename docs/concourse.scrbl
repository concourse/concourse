#lang concourse/docs

@(require "common.rkt")

@title[#:version version]{Concourse: CI that scales with your project}

@image[#:suffixes '(".svg" ".png") "images/example-pipeline"]{Example Pipeline}

Concourse is a CI system composed of simple tools and ideas. It can express
entire pipelines, integrating with arbitrary resources, or it can be used to
execute one-off tasks, either locally or in another CI system.

To see a live example, check out
@hyperlink["https://ci.concourse.ci"]{Concourse's own CI pipeline}.

Concourse is completely open source. All of the source code lives under the
@hyperlink["https://github.com/concourse"]{Concourse organization} on GitHub.
We deal with bugs and problems using GitHub Issues for each repo - please use
@hyperlink["https://github.com/concourse/concourse/issues"]{Concourse Issues}
if you're not sure. If you want to contribute patches, please use GitHub pull
requests.

For more information or if you need help: we're on IRC in the
@hyperlink["irc://irc.freenode.net/concourse"]{#concourse} channel on Freenode.
(@hyperlink["http://webchat.freenode.net/?channels=concourse"]{Webchat})

@table-of-contents[]

@include-section{what-and-why.scrbl}
@include-section{concepts.scrbl}
@include-section{getting-started.scrbl}
@include-section{fly-cli.scrbl}
@include-section{running-tasks.scrbl}
@include-section{pipelines.scrbl}
@include-section{build-plans.scrbl}
@include-section{implementing-resources.scrbl}
@include-section{release-notes.scrbl}

@inject-analytics[]
