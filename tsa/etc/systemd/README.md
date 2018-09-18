# beacon systemd service

This directory contains utilities for setting up a systemd service to
auto-register a local Garden server with the TSA.

## installation

1. Run `make install`, which copies the service unit file and the associated
binary into their respective locations.

1. Create `/etc/concourse-beacon/known_hosts`, which should contain the public
key of the target TSA. This can be created with `ssh-keyscan`, but be sure to
check the key matches what you expect!

1. Create `/etc/concourse-beacon/worker.json`, which should contain your worker
payload as documented in [Worker Pools](http://concourse-ci.org/worker-pools.html).

1. Create a SSH keypair under `/etc/concourse-beacon/keypair`, named `id_rsa`.

1. Authorize the generated key with the TSA by updating the
`tsa.authorized_keys` property in your deployment.

1. Enable an instance of the beacon, instantiated with the `host:ip` of your
TSA server:

  ```sh
  systemctl enable concourse-beacon\@ci.myserver.com:2222
  ```

1. Start the instance:

  ```sh
  systemctl start concourse-beacon\@ci.myserver.com:2222
  ```
