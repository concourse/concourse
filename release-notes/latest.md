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

#### <sub><sup><a name="4092" href="#4092">:link:</a></sup></sub> feature

* @joshzarrabi added the `--all` flag to the `fly pause-pipeline` and
`fly unpause-pipeline` commands. This allows users to pause or unpause every
pipeline on a team at the same time. #4092

[OpenTelemetry]: https://opentelemetry.io/
[Jaeger]: https://www.jaegertracing.io/
[Stackdriver]: https://cloud.google.com/trace/
