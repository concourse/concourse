# concourse

A [BOSH](https://github.com/cloudfoundry/bosh) release for concourse. The
easiest way to deploy your own instance of concourse on AWS, vSphere,
Openstack or with Vagrant.

### Project Roadmap

[Pivotal Tracker](https://www.pivotaltracker.com/n/projects/1059262)

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

    ```
    vagrant up
    ```

1. Play around with [ATC](https://github.com/concourse/atc)
  - Browse to your [local ATC](http://127.0.0.1:8080) and trigger a build.
  - Edit `manifests/vagrant-bosh.yml` and `vagrant provision` to reconfigure.

1. Play around with [Fly](https://github.com/concourse/fly)
  - Write a build config (`build.yml`) and run it with `fly`. See
    [Turbine's](https://github.com/concourse/turbine/blob/master/build.yml)
    for an example.
