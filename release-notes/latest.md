#### <sub><sup><a name="v550-note-1" href="#v550-note-1">:link:</a></sup></sub> feature

There is a new [container placement strategy](https://concourse-ci.org/container-placement.html), `limit-active-tasks`. If you specify this strategy, the cluster will maintain a counter of the number of task containers currently running on each worker. Whenever it is time to run a new container, when this strategy is in use, the worker with the fewest active tasks containers will be chosen to run it.

There is also an optional 'max active tasks per worker' configuration. If this is set to a positive integer, you will see the following behaviour: If all workers are at their active task limit, you will see the message `All workers are busy at the moment, please stand-by.` and the scheduler will re-try a minute later. This pattern will repeat each minute indefinitely, until a worker is available.

Thanks to @aledeganopix4d for all their hard work on this feature! #4118, #4148, #4208, #4277, #4142, #4221, #4293

#### <sub><sup><a name="v550-note-2" href="#v550-note-2">:link:</a></sup></sub> feature

We have changed our release notes flow! Now, contributors can add draft release notes right in their PRs, by modifying the `release-notes/latest.md` file in the `concourse/concourse` repo #4312.
