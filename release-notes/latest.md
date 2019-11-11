#### <sub><sup><a name="4688" href="#4688">:link:</a></sup></sub> feature

* The pin menu on the pipeline page now matches the sidebar, and the dropdown toggles on clicking the pin icon #4688.

#### <sub><sup><a name="4556" href="#4556">:link:</a></sup></sub> feature

* Prometheus and NewRelic can receive Lidar check-finished event now #4556.

#### <sub><sup><a name="4707" href="#4707">:link:</a></sup></sub> feature

* Make Garden client HTTP timeout configurable. #4707

#### <sub><sup><a name="4698" href="#4698">:link:</a></sup></sub> feature

* @pivotal-bin-ju @taylorsilva @xtreme-sameer-vohra added batching to the NewRelic emitter and logging info for non 2xx responses from NewRelic #4698.

#### <sub><sup><a name="4698" href="#4698">:link:</a></sup></sub> feature

* @pivotal-bin-ju @taylorsilva @xtreme-sameer-vohra added batching to the NewRelic emitter and logging info for non 2xx responses from NewRelic #4698.

#### <sub><sup><a name="4607" href="#4607">:link:</a></sup></sub> feature

* Add experimental support for exposing traces to [Jaeger].

With this feature enaled (via `--tracing-jaeger-*` variables in `concourse web`), 
the `web` node starts recording traces that represent the various steps that a
build goes through, sending them to Jaeger. #4607

As ths feature is being built using [OpenTelemetry], expect to have support for
other systems soon.

[OpenTelemetry]: https://opentelemetry.io/
[Jaeger]: https://www.jaegertracing.io/
