# Concourse Docs

This is where you will find the source for the Concourse website and overall
documentation. All of our docs are written using the Booklit documentation
engine. You can read more about its formal specification
[here](https://vito.github.io/booklit/), and you can read through its source
code [here](https://github.com/vito/booklit).

# Building the Docs Locally

## Prerequisites

* Have Go v1.8+ installed and configured. You can find the relevant
  instructions for your platform of choice here: [Go Getting
  Started](https://golang.org/doc/install)

* Clone this repository:
  [https://github.com/concourse/docs](https://github.com/concourse/docs)

## Compiling the Docs

Run the following:

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
