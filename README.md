# Concourse binary distribution

Builds a single `./concourse` binary capable of running each component of a
Concourse cluster via the following subcommands:

* `web` - runs the [ATC](https://github.com/concourse/atc), web UI and build
  scheduler, alongside a [TSA](https://github.com/concourse/tsa), used to
  securely register workers
* `worker` - runs a [Garden](https://github.com/cloudfoundry-incubator/garden)
  worker and registers it via a TSA

## Prerequisites

To run a Concourse cluster securely you'll need to generate 3 private keys:

* `host_key` - used for the TSA's SSH server. This is the key whose fingerprint
  you see when the `ssh` command warns you when connecting to a host it hasn't
  seen before.
* `worker_key` - used for authorizing worker registration. There can actually
  be an arbitrary number of these keys; they are just listed to authorize
  worker SSH access.
* `session_signing_key` (currently must be RSA) - used for signing user session
  tokens, and by the TSA to sign its own tokens in the requests it makes to the
  ATC.

To generate them, run:

```sh
ssh-keygen -t rsa -f host_key -N ''
ssh-keygen -t rsa -f worker_key -N ''
ssh-keygen -t rsa -f session_signing_key -N ''
```

...and we'll also start on an `authorized_keys` file, currently listing this
initial worker key:

```sh
cp worker_key.pub authorized_worker_keys
```

## Running the web UI and scheduler

The `concourse` binary embeds the [ATC](https://github.com/concourse/atc)
and [TSA](https://github.com/concourse/tsa) components, available as the `web`
subcommand.

The ATC is the component responsible for scheduling builds, and
also serves as the web UI and API.

The TSA provides a SSH interface for securely registering workers, even if they
live in their own private network.

### Single node, local Postgres

The following command will spin up the ATC, listening on port `8080`, with some
basic auth configured.

```sh
concourse web \
  --basic-auth-username myuser \
  --basic-auth-password mypass \
  --session-signing-key session_signing_key \
  --tsa-host-key session_signing_key \
  --tsa-authorized-keys authorized_worker_keys
```

This assumes you have a local Postgres server running on the default port
(`5432`) with an `atc` database, accessible by the current user. If your
database lives elsewhere, just specify the `--postgres-data-source` flag, which
is also demonstrated below.

### Cluster with remote Postgres

The ATC can be scaled up for high availability, and they'll also roughly share
their scheduling workloads, using the database to synchronize.

To run multiple ATCs, you'll need to pass them the following flags:

* `--postgres-data-source` should all refer to the same database
* `--peer-url` should be a URL used to reach the individual ATC, from other
  ATCs, i.e. a URL usable within their private network
* `--external-url` should be the URL used to reach *any* ATC, i.e. the URL to
  your load balancer

For example:

Node 0:

```sh
concourse web \
  --basic-auth-username myuser \
  --basic-auth-password mypass \
  --session-signing-key session_signing_key \
  --tsa-host-key session_signing_key \
  --tsa-authorized-keys authorized_worker_keys
  --postgres-data-source postgres://user:pass@10.0.32.0/concourse \
  --external-url https://ci.example.com \
  --peer-url http://10.0.16.10:8080
```

Node 1 (only difference is `--peer-url`):

```sh
concourse web \
  --basic-auth-username myuser \
  --basic-auth-password mypass \
  --session-signing-key session_signing_key \
  --tsa-host-key session_signing_key \
  --tsa-authorized-keys authorized_worker_keys
  --postgres-data-source postgres://user:pass@10.0.32.0/concourse \
  --external-url https://ci.example.com \
  --peer-url http://10.0.16.11:8080
```

## Running a worker

To spin up a [Garden](https://github.com/cloudfoundry-incubator/garden) server
and register it with your Concourse cluster at `ci.example.com`, run:

```sh
sudo concourse worker \
  --work-dir /opt/concourse/worker \
  --peer-ip 10.0.48.0 \
  --tsa-host ci.example.com \
  --tsa-public-key host_key.pub \
  --tsa-worker-private-key worker_key
```

Note that the worker must be run as `root`, as it orchestrates containers.

The `--work-dir` flag specifies where container data should be placed; make
sure it has plenty of disk space available, as it's where all the disk usage
across your builds and resources will end up.

The `--peer-ip` flag specifies the IP of this worker reachable by other `web`
nodes in your cluster. If your worker is in a private network, this flag can be
omitted, and the TSA will forward connections to the worker via a SSH tunnel
instead.

The `--tsa-host` refers to wherever your TSA node is listening, by default on
port `2222` (pass `--tsa-port` if you've configured it differently). This may
be an address to a load balancer if you're running multiple `web` nodes, or
just an IP, perhaps `127.0.0.1` if you're tinkering.

The `--tsa-public-key` flag is used to ensure we're connecting to the TSA we
should be connecting to, and is used like `known_hosts` with the `ssh` command.
Refer to [Prerequisites](#Prerequisites) if you're not sure what this means.

The `--tsa-worker-private-key` flag specifies the key to use when
authenticating to the TSA. Refer to [Prerequisites](#Prerequisites) if you're
not sure what this means.
