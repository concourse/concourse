# tsa

*controls worker authentication within concourse*

![Airport Security](https://farm4.staticflickr.com/3558/3768304342_747d4904a8_z_d.jpg)

by [stuckincustoms](https://www.flickr.com/photos/stuckincustoms/3768304342)

## about

*TSA* is the new way of allowing workers to join a Concourse deployment. It provides authentication and transport encryption (if required). Worker machines can `ssh` into *TSA* with a custom command to register or have traffic forwarded to them. Once an SSH session has been established then *TSA* begins to automatically heartbeat information about the worker into the ATC's pool.

The main advantage that this provides over the old style of registration is that Workers no longer need to be internet routable in order to have the ATC reach them. They open a reverse tunnel through the *TSA* which, when collocated with ATC, is far more likely to be easily routable. This also allows for simpler setup and better security as before you either had to expose your Garden server publicly or set up some interesting custom security if the workers and ATC were not in the same private network.

## usage

First, create two new SSH keys:

```bash
$ ssh-keygen -t rsa -f host_key
$ ssh-keygen -t rsa -f worker_key
```

Next, let's create an authorized keys file so that our workers are able to authenticate with us without providing a password:

```bash
cat worker_key.pub > authorized_keys
```

Now to start `tsa` itself:

```bash
tsa \
  --peer-ip $PEER_IP \
  --host-key ./host_key \
  --authorized-keys ./authorized_keys \
  --session-signing-key $SIGNING_KEY \
  --atc-url $ATC_URL
```

The variables here should be set to:

| Variable             | Description                                                                                               |
|----------------------|-----------------------------------------------------------------------------------------------------------|
| `$PEER_IP`           | The host or IP where this machine can be reached by the ATC for the purpose of forwarding traffic to remote workers. |
| `$SIGNING_KEY`       | RSA key used to sign the tokens used when communicating to the ATC.                                                    |
| `$ATC_URL`           | ATC URL reachable by the TSA (e.g. `https://ci.concourse.ci`).                                                        |

### registering workers

In order to have a worker on the local network register with `tsa` you can run the following command:

```bash
ssh -p 2222 $TSA_HOST \
  -i worker_key \
  -o UserKnownHostsFile=host_key.pub \
  register-worker \
  < worker.json
```

The `worker.json` file should contain the following:

```json
{
    "platform": "linux",
    "tags": [],
    "addr": "$GARDEN_ADDR",
    "baggageclaim_url": "$BAGGAGECLAIM_URL"
}
```

The variables here should be set to:

| Variable             | Description                                                             |
|----------------------|-------------------------------------------------------------------------|
| `$TSA_HOST`          | The hostname or IP where the TSA server can be reached.                 |
| `$GARDEN_ADDR`       | The address (host and port) of the Garden to advertise.                 |
| `$BAGGAGECLAIM_URL`  | The API URL (scheme, host,  and port) of the BaggageClaim to advertise. |


### forwarding workers

In order to have a worker on a remote network register with `tsa` and have its traffic forwarded you can run the following command:

```bash
ssh -p 2222 $TSA_HOST \
  -i worker_key \
  -o UserKnownHostsFile=host_key.pub \
  -R0.0.0.0:7777:127.0.0.1:7777 \
  -R0.0.0.0:7788:127.0.0.1:7788 \
  forward-worker \
    --garden 0.0.0.0:7777 \
    --baggageclaim 0.0.0.0:7788 \
  < worker.json
```

Note that in this case you should always have Garden and BaggageClaim listen on `127.0.0.1` so that they're not exposed to the outside world. For this reason there is no `$GARDEN_ADDR` or `$BAGGAGECLAIM_URL` as is the case with `register-worker`.

The `worker.json` file should contain the following:

```json
{
    "platform": "linux",
    "tags": []
}
```
