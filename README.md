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
tsa -forwardHost=$FORWARD_HOST \
      -hostKey=host_key \
      -authorizedKeys=authorized_keys \
      -heartbeatInterval=30s \
      -atcAPIURL=http://$USERNAME:$PASSWORD@$ATC_HOST:$ATC_PORT
```

The variables here should be set to:

| Variable             | Description                                                                                               |
|----------------------|-----------------------------------------------------------------------------------------------------------|
| `$FORWARD_HOST`      | The host or IP where this machine can be reached for the purpose of forwarding traffic to remote workers. |
| `$USERNAME`          | Username for the ATC                                                                                      |
| `$PASSWORD`          | Password for the ATC                                                                                      |
| `$ATC_HOST`          | Host for the ATC                                                                                          |
| `$ATC_PORT`          | Port for the ATC                                                                                          |

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
    "resource_types": []
}
```

This should be set to whatever you want to advertise

The variables here should be set to:

| Variable             | Description                                             |
|----------------------|---------------------------------------------------------|
| `$TSA_HOST`          | The hostname or IP where the TSA server can be reached. |
| `$GARDEN_ADDR`       | The address (host and port) of the Garden to advertise. |

### forwarding workers

In order to have a worker on a remote network register with `tsa` and have it's traffic forwarded you can run the following command:

```bash
ssh -p 2222 $TSA_HOST \
      -i worker_key \
      -o UserKnownHostsFile=host_key.pub \
      -R0.0.0.0:0:$GARDEN_ADDR \
      forward-worker \
      < worker.json
```

The `worker.json` file should contain the following:

```json
{
    "platform": "linux",
    "tags": [],
    "resource_types": []
}
```

This should be set to whatever you want to advertise

The variables here should be set to:

| Variable             | Description                                             |
|----------------------|---------------------------------------------------------|
| `$TSA_HOST`          | The hostname or IP where the TSA server can be reached. |
| `$GARDEN_ADDR`       | The address (host and port) of the Garden to advertise. |
