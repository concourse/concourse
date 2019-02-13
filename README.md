# Concourse Docs

This is where you will find the source for the Concourse website and overall
documentation. All of our docs are written using the [Booklit documentation
engine](https://vito.github.io/booklit/).

**Table of Contents**
* [Building the Docs Locally](#building-the-docs-locally)
* [Docs Styling](#docs-styling)
* [Content Layout](#content-layout)

# Examples Status

- git-triggered [![](https://ci.concourse-ci.org/api/v1/teams/examples/pipelines/git-triggered/badge)](https://ci.concourse-ci.org/teams/examples/pipelines/git-triggered)
- golang [![](https://ci.concourse-ci.org/api/v1/teams/examples/pipelines/golang/badge)](https://ci.concourse-ci.org/teams/examples/pipelines/golang)
- hooks [![](https://ci.concourse-ci.org/api/v1/teams/examples/pipelines/hooks/badge)](https://ci.concourse-ci.org/teams/examples/pipelines/hooks)
- java [![](https://ci.concourse-ci.org/api/v1/teams/examples/pipelines/java/badge)](https://ci.concourse-ci.org/teams/examples/pipelines/java)
- job [![](https://ci.concourse-ci.org/api/v1/teams/examples/pipelines/job/badge)](https://ci.concourse-ci.org/teams/examples/pipelines/job)
- nodejs [![](https://ci.concourse-ci.org/api/v1/teams/examples/pipelines/nodejs/badge)](https://ci.concourse-ci.org/teams/examples/pipelines/nodejs)
- parameters [![](https://ci.concourse-ci.org/api/v1/teams/examples/pipelines/parameters/badge)](https://ci.concourse-ci.org/teams/examples/pipelines/parameters)
- php [![](https://ci.concourse-ci.org/api/v1/teams/examples/pipelines/php/badge)](https://ci.concourse-ci.org/teams/examples/pipelines/php)
- rails [![](https://ci.concourse-ci.org/api/v1/teams/examples/pipelines/rails/badge)](https://ci.concourse-ci.org/teams/examples/pipelines/rails)
- serial-job [![](https://ci.concourse-ci.org/api/v1/teams/examples/pipelines/serial-job/badge)](https://ci.concourse-ci.org/teams/examples/pipelines/serial-job)
- task-passing-artifact [![](https://ci.concourse-ci.org/api/v1/teams/examples/pipelines/task-passing-artifact/badge)](https://ci.concourse-ci.org/teams/examples/pipelines/task-passing-artifact)
- time-triggered [![](https://ci.concourse-ci.org/api/v1/teams/examples/pipelines/time-triggered/badge)](https://ci.concourse-ci.org/teams/examples/pipelines/time-triggered)
- trigger-then-continue [![](https://ci.concourse-ci.org/api/v1/teams/examples/pipelines/trigger-then-continue/badge)](https://ci.concourse-ci.org/teams/examples/pipelines/trigger-then-continue)

# Building the Docs Locally

## Prerequisites

* Have Go v1.11.2+ installed and configured. You can find the relevant
  instructions for your platform of choice here: [Go Getting
  Started](https://golang.org/doc/install)

* Clone this repository:
  [https://github.com/concourse/docs](https://github.com/concourse/docs)

## Compiling the Docs

You can compile the Concourse docs by running:

```bash
./scripts/build
```

The `build` script will instruct Booklit to compile all the files under `lit/`
as `html` files. The files will then be dumped into your current working
directory, i.e. the root of this repo.

## Viewing the docs in your browser

To run a server that will rebuild the docs as needed, pass `-s (port)` like so:

```bash
./scripts/build -s 8000
```

You will be now be able to see the rendered site if you navigate to
[http://localhost:8000](http://localhost:8000).

# Docs Styling
You can find all of the styling assets for the Concourse website and documentation under the [`css/`](https://github.com/concourse/docs/tree/master/css) folder. 

If you are planning to make changes to the site, [`css/booklit.css`](https://github.com/concourse/docs/blob/master/css/booklit.css) is usually a good place to start. 

# Content Layout
All of the website content can be found under the [`lit/`](https://github.com/concourse/docs/tree/master/lit) folder of this repository. 

The content layout for the site is qute simple, and for the most part self-explanatory. If you want to change a specific page on the website you can usually jump straight to it by looking for the `.lit` version of the page. For example you can make changes to https://concourse-ci.org/fly.html by editing `lit/fly.lit`. 

* **`html/docs-header.tmpl`** L1 navigation header for the Concourse website and docs.
* **`lit/index.html`** The Concourse Homepage
* **`lit/reference/`** This is where you'll find most of the documentation listed under https://concourse-ci.org/reference.html
* **`lit/release-notes/`** Release notes separated by Concourse version. These `.lit` snippets are ultimately loaded into `lit/download.lit`
* **`lit/download.lit/`** Concourse Downloads page
* **`lit/docs/resource-types/community-resources.lit`** A listing of Concourse community-supported resources
