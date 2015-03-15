#lang scribble/manual

@(require "common.rkt")

@title[#:version version #:tag "implementing-resources"]{Implementing a Resource}

A resource type is implemented by a container image with three scripts:

@itemlist[
  @item{
    @code{/opt/resource/check} for checking for new versions of the resource
  }

  @item{
    @code{/opt/resource/in} for pulling a version of the resource down
  }

  @item{
    @code{/opt/resource/out} for idempotently pushing a version up
  }
]

Distributing resource types as containers allows them to package their own
dependencies. For example, the Git resource comes with @code{git} installed.

If a resource will only ever be used for generating output (for example, code
coverage), it's reasonable to only implement @code{out}. This will work just
fine so long as no one tries to use it as an input.

For use as an input, a resource should always implement @code{in} and
@code{check} together.

@section{@code{check}: Check for new versions.}

A resource type's @code{check} script is invoked to detect new versions of
the resource. It is given the configured source and current version on stdin,
and must print the array of new versions, in chronological order, to stdout.

Note that the current version will be missing if this is the first time the
resource has been used. In this case, the script should emit only the most
recent version, @emph{not} every version since the resource's inception.

For example, here's what the input for a @code{git} resource may look like:

@codeblock|{
{
  "source": {
    "uri": "git://some-uri",
    "branch": "develop",
    "private_key": "..."
  },
  "version": { "ref": "61cebf" }
}
}|

Upon receiving this payload the @code{git} resource would probably do
something like:

@codeblock|{
[ -d /tmp/repo ] || git clone git://some-uri /tmp/repo
cd /tmp/repo
git pull && git log 61cbef..HEAD
}|

Note that it conditionally clones; the container for checking versions is reused
between checks, so that it can efficiently pull rather than cloning every time.

And the output, assuming @code{d74e01} is the commit immediately after
@code{61cbef}:

@codeblock|{
[
  { "ref": "d74e01" },
  { "ref": "7154fe" }
]
}|

The list may be empty, if the given version is already the latest.

@section{@code{in}: Fetch a given resource.}

The @code{in} script is passed a destination directory as @code{$1}, and
is given on stdin the configured source and, optionally, a precise version of
the resource to fetch.

The script must fetch the resource and place it in the given directory.

Because the input may not specify a version, the @code{in} script must print
out the version that it fetched. This allows the upstream to not have to perform
@code{check} before @code{in}, which can be slow (for git it implies two
clones).

Additionally, the script may emit metadata as a list of key-value pairs. This
data is intended for public consumption and will make it upstream, intended to
be shown on the build's page.

Example input, in this case for the @code{git} resource:

@codeblock|{
{
  "source": {
    "uri": "git://some-uri",
    "branch": "develop",
    "private_key": "..."
  },
  "version": { "ref": "61cebf" }
}
}|

Note that the @code{version} may be @code{null}.

Upon receiving this payload the @code{git} resource would probably do
something like:

@codeblock|{
git clone --branch develop git://some-uri $1
cd $1
git checkout 61cebf
}|

And output:

@codeblock|{
{
  "version": { "ref": "61cebf" },
  "metadata": [
    { "name": "commit", "value": "61cebf" },
    { "name": "author", "value": "Hulk Hogan" }
  ]
}
}|


@section{@code{out}: Update a resource.}

The @code{out} script is called with a path to the directory containing the
build's full set of sources as the first argument, and is given on stdin the
configured params and the resource's source information. The source directory
is as it was at the end of the build.

The script must emit the resulting version of the resource. For example, the
@code{git} resource emits the sha of the commit that it just pushed.

Additionally, the script may emit metadata as a list of key-value pairs. This
data is intended for public consumption and will make it upstream, intended to
be shown on the build's page.

Example input, in this case for the @code{git} resource:

@codeblock|{
{
  "params": {
    "branch": "develop",
    "repo": "some-repo"
  },
  "source": {
    "uri": "git@...",
    "private_key": "..."
  }
}
}|

Upon receiving this payload the @code{git} resource would probably do something
like:

@codeblock|{
cd $1/some-repo
git push origin develop
}|

And output:

@codeblock|{
{
  "version": { "ref": "61cebf" },
  "metadata": [
    { "name": "commit", "value": "61cebf" },
    { "name": "author", "value": "Mick Foley" }
  ]
}
}|

@inject-analytics[]
