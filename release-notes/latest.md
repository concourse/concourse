#### <sub><sup><a name="v550-note-1" href="#v550-note-1">:link:</a></sup></sub> feature

* There is a new [container placement strategy](https://concourse-ci.org/container-placement.html), `limit-active-tasks`. If you specify this strategy, the cluster will maintain a counter of the number of task containers currently running on each worker. Whenever it is time to run a new container, when this strategy is in use, the worker with the fewest active tasks containers will be chosen to run it.
  There is also an optional 'max active tasks per worker' configuration. If this is set to a positive integer, you will see the following behaviour: If all workers are at their active task limit, you will see the message `All workers are busy at the moment, please stand-by.` and the scheduler will re-try a minute later. This pattern will repeat each minute indefinitely, until a worker is available.
  Thanks to @aledeganopix4d for all their hard work on this feature! #4118, #4148, #4208, #4277, #4142, #4221, #4293, #4161, #4315

#### <sub><sup><a name="v550-note-2" href="#v550-note-2">:link:</a></sup></sub> feature

* We have changed our release notes flow! Now, contributors can add draft release notes right in their PRs, by modifying the `release-notes/latest.md` file in the `concourse/concourse` repo #4312.

#### <sub><sup><a name="v550-note-3" href="#v550-note-3">:link:</a></sup></sub> feature

* In the past, [admins](https://concourse-ci.org/user-roles.html#concourse-admin) (owners of the `main` team) had permission to modify the auth configuration for other teams in the same cluster. Now, admins also have full control over pipelines, jobs, resources, builds, etc for all teams. Using `fly`, they can log in to any team on the cluster as though they are an owner #4238, #4273.

#### <sub><sup><a name="v550-note-4" href="#v550-note-4">:link:</a></sup></sub> feature
* We noticed after #4058 (where build steps are collapsed by default) that it wasn't very easy to see failing steps.
  Now a failing step has a red border around its header, an erroring step has an orange border, and a running step has a yellow border. #4164, #4250

#### <sub><sup><a name="v550-note-5" href="#v550-note-5">:link:</a></sup></sub> feature

* On particularly busy clusters, users have observed [metrics events](https://github.com/concourse/concourse/issues/3674) [being dropped](https://github.com/concourse/concourse/issues/3769) due to a full queue #3937. @rudolfv added a configurable buffer size for metrics emission, regardless of your configured emitter. This should allow operators to trade memory pressure on the web nodes for reliability of metric transmission.

#### <sub><sup><a name="v550-note-6" href="#v550-note-6">:link:</a></sup></sub> feature

* @rudolfv also added some features for the special case of InfluxDB metrics. To decrease the request load on InfluxDB, you can configure the number of events to batch into a single InfluxDB request, or you can specify a hardcoded interval at which to emit events, regardless of how many have accumulated #3937.

#### <sub><sup><a name="v550-note-7" href="#v550-note-7">:link:</a></sup></sub> fix

* @evanchaoli improved `fly` - when outputting sample commands to the terminal, the CLI is aware of the path from which it is being executed #4284.

#### <sub><sup><a name="v550-note-8" href="#v550-note-8">:link:</a></sup></sub> fix

* The web UI used to [silently break](https://github.com/concourse/concourse/issues/3141) when your token (which includes a potentially-long JSON-encoded string detailing all the teams you are part of and what roles you have on them) was longer than the size of a single cookie (4096 bytes on most browsers!). This limit has been increased 15-fold, which should unblock most users on clusters with a lot of teams #4280.

#### <sub><sup><a name="v550-note-9" href="#v550-note-9">:link:</a></sup></sub> fix

* For the past few releases, the web nodes have allowed themselves to make up to 64 parallel connections to the database, to allow for parallelizing work like GC and scheduling within a single node. @ebilling has configured the web node's tolerance for idle connections to be more lenient: If a node has been using more than 32 of its available connections, up to 32 connections will be allowed to stay idly open. Anecdotally, CPU savings (resulting from less opening/closing of connections) of up to 30% have been observed on web nodes because of this change. Furthermore, the total max connection pool size has been made configurable - this should allow operators to avoid overloading the max connection limit on the database side #4232.

#### <sub><sup><a name="v550-note-10" href="#v550-note-10">:link:</a></sup></sub> fix

* @josecv found and fixed a subtle bug where, if you had a [`try`](https://concourse-ci.org/try-step.html) step and you aborted while the hooked step was running, your whole web node would [crash](https://github.com/concourse/concourse/issues/3989)! Good catch #4252.

#### <sub><sup><a name="v550-note-11" href="#v550-note-11">:link:</a></sup></sub> fix

* @aledeganopix4d fixed an [issue](https://github.com/concourse/concourse/issues/4180) where the logs for a Windows or Darwin worker get populated with irrelevant error messages #4167.

#### <sub><sup><a name="v550-note-12" href="#v550-note-12">:link:</a></sup></sub> feature

* @nazrhom improved the output of `fly targets` to show an error message in the table if your token for a given target is invalid #4181, #4228.

#### <sub><sup><a name="v550-note-13" href="#v550-note-13">:link:</a></sup></sub> fix

* Since introducing [Zstandard compression for volume streaming](https://github.com/concourse/concourse/releases#v540-note-1), we noticed a [new class of baggageclaim errors](https://github.com/concourse/retryhttp/issues/8) saying `http: unexpected EOF reading trailer` cropping up in our own CI environment, so we updated our http clients to retry requests on this error #4233.

#### <sub><sup><a name="v550-note-14" href="#v550-note-14">:link:</a></sup></sub> feature

* Concourse admins can now run [`fly active-users`](https://concourse-ci.org/managing-teams.html#fly-active-users) and get a summary of all the users on the cluster, filtering by their last login time (the last 2 months by default) #4096.
