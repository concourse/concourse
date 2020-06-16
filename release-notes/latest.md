#### <sub><sup><a name="5780" href="#5780">:link:</a></sup></sub> fix

* Fixed a regression where builds could be stuck pending forever if an input was pinned by partially specifying a version via the [`version:` on a `get` step](https://concourse-ci.org/jobs.html#schema.step.get-step.version), [`version:` on a resource config](https://concourse-ci.org/resources.html#schema.resource.version) or by running [`fly pin-resource`](https://concourse-ci.org/managing-resources.html#fly-pin-resource). #5780
