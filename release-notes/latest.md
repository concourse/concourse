#### <sub><sup><a name="4688" href="#4688">:link:</a></sup></sub> feature

* The pin menu on the pipeline page now matches the sidebar, and the dropdown toggles on clicking the pin icon #4688.

#### <sub><sup><a name="4556" href="#4556">:link:</a></sup></sub> feature

* Prometheus and NewRelic can receive Lidar check-finished event now #4556.

#### <sub><sup><a name="4707" href="#4707">:link:</a></sup></sub> feature

* Make Garden client HTTP timeout configurable. #4707

#### <sub><sup><a name="4698" href="#4698">:link:</a></sup></sub> feature

* @pivotal-bin-ju @taylorsilva @xtreme-sameer-vohra added batching to the NewRelic emitter and logging info for non 2xx responses from NewRelic #4698.

#### <sub><sup><a name="4748" href="#4748">:link:</a></sup></sub> feature

* @andhadley added support for Vault namespaces. #4748

#### <sub><sup><a name="4749" href="#4749">:link:</a></sup></sub> fix

* @evanchaoli fixed a weird behavior with secret redaction wherein a secret containing e.g. `{` on its own line (i.e. formatted JSON) would result in `{` being replaced with `((redacted))` in build logs. Single-character lines will instead be skipped.

  As an aside, anyone with a truly single-character credential *may* want to add another character or two.

#### <sub><sup><a name="4804" href="#4804">:link:</a></sup></sub> fix

* @vito bumped the `autocert` dependency so that Let's Encrypt will default to the ACME v2 API. #4804

#### <sub><sup><a name="registry-image-0.8.2" href="#registry-image-0.8.2">:link:</a></sup></sub> fix

* Bumped the [`registry-image` resource](https://github.com/concourse/registry-image-resource) to [v0.8.2](https://github.com/concourse/registry-image-resource/releases/tag/v0.8.2), which should resolve `DIGEST_INVALID` errors (among others) introduced by faulty retry logic. Additionally, the resource will now retry on `429 Too Many Requests` errors from the registry, with exponential back-off up to 1 hour.

#### <sub><sup><a name="4808" href="#4808">:link:</a></sup></sub> fix

* @evanchaoli fixed a race condition resulting in a crash with LIDAR enabled. #4808

#### <sub><sup><a name="4817" href="#4817">:link:</a></sup></sub> fix

* @evanchaoli fixed a regression introduced with the secret redaction work which resulted in build logs being buffered. #4817
