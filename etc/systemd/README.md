# beacon systemd service

This directory contains utilities for setting up a systemd service to
auto-register a local Garden server with the TSA.

## installation

1. Copy the `concourse-beacon@.service` unit template into your systemd units
directory, e.g. `/etc/systemd/systemd`. Note that `systemd link` won't work for
the later steps; seems to be a systemd bug.

1. Copy `concourse-beacon` into `/usr/sbin`.

1. Create `/etc/concourse-beacon/known_hosts`, which should contain the public
key of the target TSA. This can be created with `ssh-keyscan`, but be sure to
check the key matches what you expect!

1. Create `/etc/concourse-beacon/worker.json`, which should contain your worker
payload as documented in [Worker Pools](http://concourse.ci/worker-pools.html).

1. Create a SSH keypair under `/etc/concourse-beacon/keypair`, named `id_rsa`.

1. Enable an instance of the beacon, instantiated with the `host:ip` of your
TSA server:

  ```sh
  systemctl enable concourse-beacon\@ci.myserver.com:2222
  ```

1. Start the instance:

  ```sh
  systemctl start concourse-beacon\@ci.myserver.com:2222
  ```
