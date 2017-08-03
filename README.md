# Concourse Docs
This is where you will find the source for the Concourse website and overall documentation. All of our docs are written using the Booklit documentation engine. You can read more about its formal specification [here](https://vito.github.io/booklit/), and you can read through its source code [here](https://github.com/vito/booklit). 

# Building the Docs Locally
## Prerequisites
* Have Go v1.8+ installed and configured. You can find the relevant instructions for your platform of choice here: [Go Getting Started](https://golang.org/doc/install) 
* Python2 or Python3 installed and configured
* (Optional) Install Python [VirtualEnv](https://virtualenv.pypa.io/en/stable/) and [VirtualEnvWrapper ](https://virtualenvwrapper.readthedocs.io/)
* Make a clone of the `concourse` repository: [https://github.com/concourse/concourse](https://github.com/concourse/concourse)

## Install the Packages
Install [**booklit**](https://github.com/vito/booklit): 

```bash
go get github.com/vito/booklit/cmd/booklit
```

Install [**semver**](https://github.com/blang/semver)

```bash
go get github.com/blang/semver
```

Install [**pygments**](http://pygments.org/)

```bash
sudo pip install pygments
```

or, if you have `virtualenv` and `virtualenvwrapper` installed

```bash
mkvirtualenv concourse-docs
workon concourse-docs
pip install pygments
```

## Compiling the Docs
From project root, we're going to move into the `docs` folder:

```bash
cd docs
```

and from here we'll use the build script to compile our `.lit` files:

```bash
# If you installed pygments under a virtualenv, make sure to switch 
# into it now before you execute the script

./scripts/build
```

The `build` script will instruct booklit to compile all the files under `lit/` as `html` files.
The files will then be dumped into your current working directory, in this case its `docs/`

## Viewing the docs in your browser

To run a server that will rebuild the docs as needed, pass `-s (port)` like so:

```bash
./scripts/build -s 8000
```

You will be now be able to see the rendered site if you navigate to [http://localhost:8000](http://localhost:8000)
