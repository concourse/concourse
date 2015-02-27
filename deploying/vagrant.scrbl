#lang scribble/manual

@(require "../common.rkt")

@title[#:version version #:tag "deploying-with-vagrant"]{Provisioning with Vagrant}

Before you go and spend money on hardware, you probably want to kick the tires a
bit.

The easiest way to get something up and running is to use the pre-built Vagrant
boxes. This requires no BOSH experience, but is otherwise exactly the same as a
BOSH deployment in terms of features.

Currently, only VirtualBox is supported. Pre-built AWS and VMware Fusion boxes
will be available in the future.

@; If you have no need for multiple workers, you can use a Vagrant provider like
@; AWS to spin up a single-instance deployment. This is perfectly fine for a lot
@; of use cases.

To spin up Concourse with Vagrant, run the following in any directory:

@codeblock|{
$ vagrant init concourse/lite ; # places Vagrantfile in current directory
$ vagrant up                  ; # downloads the box and spins up the VM
}|

The web server will be running at @hyperlink["http://192.168.100.4:8080"]{192.168.100.4:8080}.

Next, download the Fly CLI from the @hyperlink["http://192.168.100.4:8080"]{main page}.
There are links to binaries for common platforms at the bottom right.

@margin-note{
  If you're on Linux or OS X, you will have to @code{chmod +x} the downloaded
  binary and put it in your @code{$PATH}.
}

The default pipeline configuration is blank. You can see the current configuration by running:

@codeblock|{
$ fly configure
}|

To configure a pipeline, fill in jobs and resources (see @secref{pipelines}),
place the resulting config in a file, and run:

@codeblock|{
$ fly configure -c path/to/pipeline.yml
}|

This will validate your pipeline config and immediately switch Concourse to it.

To run arbitrary builds against the Vagrant box, see @secref{running-builds}.
