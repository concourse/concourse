#lang scribble/manual

@title[#:style '(quiet unnumbered)]{v0.43.0}

@itemlist[
  @item{
    Two new resources:
    @hyperlink["https://github.com/concourse/bosh-io-release-resource"]{bosh.io
    release} and
    @hyperlink["https://github.com/concourse/bosh-io-stemcell-resource"]{bosh.io
    stemcell}, for consuming public BOSH releases and stemcells in a more
    convenient way.
  }

  @item{
    The event stream is now GZip compressed, which should speed up build logs.
  }

  @item{
    The @hyperlink["https://github.com/concourse/s3-resource"]{S3 resource} now
    supports creating URLs using CloudFront.
  }

  @item{
    The @hyperlink["https://github.com/concourse/git-resource"]{Git resource}
    can now create tags while pushing.
  }

  @item{
    Autoscrolling was broken. It's fixed now.
  }
]
