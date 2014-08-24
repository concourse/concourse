#lang scribble/manual

@title[#:style 'toc]{Getting Started}

The @hyperlink["https://github.com/concourse/concourse"]{Concourse repo} is a
@hyperlink["https://github.com/cloudfoundry/bosh"]{BOSH} release containing
everything necessary to deploy to arbitrary infrastructures (or your laptop)
with largely the same configuration.

There are two ways to get started with Concourse:

@itemlist[
  @item{
    @seclink["deploying-with-vagrant"]{With Vagrant} - for a quick deploy with
    familiar tooling.
  }

  @item{
    @seclink["deploying-with-bosh"]{With BOSH} - for a scalable cluster, managed
    by BOSH.
  }
]

@include-section{deploying/vagrant.scrbl}
@include-section{deploying/bosh.scrbl}
