#### <sub><sup><a name="v550-note-1" href="#v550-note-1">:link:</a></sup></sub> feature

There is a new [container placement strategy](https://concourse-ci.org/container-placement.html), `limit-active-tasks`. If you specify this strategy, the cluster will maintain a counter of the number of task containers currently running on each worker. Whenever it is time to run a new container, when this strategy is in use, the worker with the fewest active tasks containers will be chosen to run it.

There is also an optional 'max active tasks per worker' configuration. If this is set to a positive integer, you will see the following behaviour: If all workers are at their active task limit, you will see the message `All workers are busy at the moment, please stand-by.` and the scheduler will re-try a minute later. This pattern will repeat each minute indefinitely, until a worker is available.

Thanks to @aledeganopix4d for all their hard work on this feature! #4118, #4148, #4208, #4277, #4142, #4221, #4293, #4161

#### <sub><sup><a name="v550-note-2" href="#v550-note-2">:link:</a></sup></sub> feature

We have changed our release notes flow! Now, contributors can add draft release notes right in their PRs, by modifying the `release-notes/latest.md` file in the `concourse/concourse` repo #4312.

#### <sub><sup><a name="v550-note-3" href="#v550-note-3">:link:</a></sup></sub> feature

In the past, owners of the `main` team had permission to modify the auth configuration for other teams in the same cluster. Now, owners of the `main` team also have full control over pipelines, jobs, resources, builds, etc for all teams #4238.

#### <sub><sup><a name="v550-note-4" href="#v550-note-4">:link:</a></sup></sub> feature
We noticed after #4058 (where build steps are collapsed by default) that it wasn't very easy to see failing steps.

Now a failing step has a red border around its header, an erroring step has an orange border, and a running step has a yellow border. #4164, #4250

#### <sub><sup><a name="v550-note-5" href="#v550-note-5">:link:</a></sup></sub> feature

On particularly busy clusters, users have observed [metrics events](https://github.com/concourse/concourse/issues/3674) [being dropped](https://github.com/concourse/concourse/issues/3769) due to a full queue #3937. @rudolfv added a configurable buffer size for metrics emission, regardless of your configured emitter. This should allow operators to trade memory pressure on the web nodes for reliability of metric transmission.

#### <sub><sup><a name="v550-note-6" href="#v550-note-6">:link:</a></sup></sub> feature

@rudolfv also added some features for the special case of InfluxDB metrics. To decrease the request load on InfluxDB, you can configure the number of events to batch into a single InfluxDB request, or you can specify a hardcoded interval at which to emit events, regardless of how many have accumulated #3937.

#### <sub><sup><a name="v550-note-7" href="#v550-note-7">:link:</a></sup></sub> fix

@evanchaoli improved `fly` - when outputting sample commands to the terminal, the CLI is aware of the path from which it is being executed #4284.
