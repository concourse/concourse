#### <sub><sup><a name="4688" href="#4688">:link:</a></sup></sub> feature

* The pin menu on the pipeline page now matches the sidebar, and the dropdown toggles on clicking the pin icon #4688.

#### <sub><sup><a name="4556" href="#4556">:link:</a></sup></sub> feature

* Prometheus and NewRelic can receive Lidar check-finished event now #4556.

#### <sub><sup><a name="4707" href="#4707">:link:</a></sup></sub> feature

* Make Garden client HTTP timeout configurable. #4707

#### <sub><sup><a name="4698" href="#4698">:link:</a></sup></sub> feature

* @pivotal-bin-ju @taylorsilva @xtreme-sameer-vohra added batching to the NewRelic emitter and logging info for non 2xx responses from NewRelic #4698.

#### <sub><sup><a name="4749" href="#4749">:link:</a></sup></sub> fix

* @evanchaoli fixed a weird behavior with secret redaction wherein a secret containing e.g. `{` on its own line (i.e. formatted JSON) would result in `{` being replaced with `((redacted))` in build logs. Single-character lines will instead be skipped.

  As an aside, anyone with a truly single-character credential *may* want to add another character or two.

#### <sub><sup><a name="4804" href="#4804">:link:</a></sup></sub> fix

* @vito bumped the `autocert` dependency so that Let's Encrypt will default to the ACME v2 API. #4804

#### <sub><sup><a name="Registry Image Resource">:link:</a></sup></sub> fix

* Drop hashicorp/go-retryablehttp in favor of an outer retryer to avoid mismatched digests during puts
