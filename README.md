# concourse

A [BOSH](https://github.com/cloudfoundry/bosh) release for concourse. The
easiest way to deploy your own instance of concourse on AWS, vSphere,
Openstack or with Vagrant.

* Documentation: [concourse.ci](http://concourse.ci)
* IRC: `#concourse` on Freenode
* Roadmap: [Pivotal Tracker](https://www.pivotaltracker.com/n/projects/1059262)


### Try it on Vagrant

1. Fetch the bosh-release

   ```
   git clone https://github.com/concourse/concourse
   cd concourse
   git submodule update --init --recursive
   ```

1. Install dependencies

    ```
    vagrant plugin install vagrant-bosh
    gem install bosh_cli --no-ri --no-rdoc
    go get github.com/concourse/fly
    ```

1. Create a new VM

    This can take a bit as it will compile everything from source (including
    Postgres). Later provisions won't take nearly as long.

    ```
    vagrant up --provider virtualbox
    ```

    Currently the only supported provider is VirtualBox. Other providers (AWS,
    VMware Fusion) should work with minimal changes, but are not tested.

1. Play around with [ATC](https://github.com/concourse/atc), the web UI.
  - Browse to your [local ATC](http://127.0.0.1:8080) and trigger a build.
  - Edit `manifests/vagrant-bosh.yml` and `vagrant provision` to reconfigure.

1. Play around with [Fly](https://github.com/concourse/fly), the CLI.
  - Write a [build config](http://concourse.ci/running-builds.html) and run it with `fly`.
  - Hop into the container of a running build with `fly hijack`.
