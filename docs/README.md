# Authoring Docs

The docs are written in [Anatomy](https://github.com/vito/anatomy), which
admittedly has no docs of its own. Your best bet if you don't know it yet is to
just wing it and look to the rest of the docs for pointers.

# Building Docs

Building the docs can either be done locally with Anatomy or done remotely via
`fly`. Getting everything going locally is much better but it's a bit harder to
get Rubinius installed.

## Locally

1. Install [Rubinius](https://rubinius.com/).
1. `bundle install`
1. `bundle exec anatomy -i index.any -o /tmp/docs`
1. Open `/tmp/docs/index.html` in your browser.

Alternatively, pass `-s` to `anatomy` and it'll spin up a web server that
rebuilds and serves the docs live.

## With `fly`

1. Spin up a Concourse somewhere.
1. From the `concourse` repo, run `fly -t TARGET execute -x -c ci/build-docs.yml -o built-docs=/tmp/docs`
1. Open `/tmp/docs/index.html` in your browser.
