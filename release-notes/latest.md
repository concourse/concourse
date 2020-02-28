#### <sub><sup><a name="4938" href="#4938">:link:</a></sup></sub> feature

* Include job label in build duration metrics exported to Prometheus. #4976

#### <sub><sup><a name="5023" href="#5023">:link:</a></sup></sub> fix

* The dashboard page refreshes its data every 5 seconds. Until now, it was possible (especially for admin users) for the dashboard to initiate an ever-growing number of API calls, unnecessarily consuming browser, network and API resources. Now the dashboard will not initiate a request for more data until the previous request finishes. #5023

#### <sub><sup><a name="4607" href="#4607">:link:</a></sup></sub> feature

* Add experimental support for exposing traces to [Jaeger] or [Stackdriver].

With this feature enabled (via `--tracing-(jaeger|stackdriver)-*` variables in
`concourse web`), the `web` node starts recording traces that represent the
various steps that a build goes through, sending them to the configured trace
collector. #4607

As this feature is being built using [OpenTelemetry], expect to have support for
other systems soon.

[OpenTelemetry]: https://opentelemetry.io/
[Jaeger]: https://www.jaegertracing.io/
[Stackdriver]: https://cloud.google.com/trace/

#### <sub><sup><a name="4092" href="#4092">:link:</a></sup></sub> feature

* @joshzarrabi added the `--all` flag to the `fly pause-pipeline` and
`fly unpause-pipeline` commands. This allows users to pause or unpause every
pipeline on a team at the same time. #4092

#### <sub><sup><a name="5133" href="#5133">:link:</a></sup></sub> fix

* In the case that a user has multiple roles on a team, the pills on the team headers on the dashboard now accurately reflect the logged-in user's most-privileged role on each team. #5133

#### <sub><sup><a name="5118" href="#5118">:link:</a></sup></sub> feature

* Improved the performance of the dashboard by only rendering the pipeline cards that are visible. #5118

#### <sub><sup><a name="5160" href="#5160">:link:</a></sup></sub> fix

* Fix misuse of mount options when performing copy-on-write volumes based on
  other copy-on-write volumes 

This case could be faced when providing inputs and outputs with
overlapping paths.

* Switch CGO-based Zstd library by a pure go one

Certain payloads could make Concourse return internal errors due to possible
errors from the library we used before.

#### <sub><sup><a name="4847" href="#4847">:link:</a></sup></sub> fix

* Set a default value of `4h` for `rebalance-interval`. Previously, this value was unset. With the new default, the workers will reconnect to a randomly selected TSA (SSH Gateway) every 4h.

#### <sub><sup><a name="5216" href="#5216">:link:</a></sup></sub> fix

* Bump golang.org/x/crypto module from `v0.0.0-20191119213627-4f8c1d86b1ba` to `v0.0.0-20200220183623-bac4c82f6975` to address vulnerability in ssh package.

#### <sub><sup><a name="5148" href="#5148">:link:</a></sup></sub> feature

* Improve the initial page load time by lazy-loading Javascript that isn't necessary for the first render. #5148

[OpenTelemetry]: https://opentelemetry.io/
[Jaeger]: https://www.jaegertracing.io/
[Stackdriver]: https://cloud.google.com/trace/
