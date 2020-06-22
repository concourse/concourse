#### <sub><sup><a name="5780" href="#5780">:link:</a></sup></sub> fix

* Fixed a regression where builds could be stuck pending forever if an input was pinned by partially specifying a version via the [`version:` on a `get` step](https://concourse-ci.org/jobs.html#schema.step.get-step.version), [`version:` on a resource config](https://concourse-ci.org/resources.html#schema.resource.version) or by running [`fly pin-resource`](https://concourse-ci.org/managing-resources.html#fly-pin-resource). #5780

#### <sub><sup><a name="5758" href="#5758">:link:</a></sup></sub> fix

* @evanchaoli fixed a regression that prevented using both [static vars] and [dynamic vars] simultaneously in a task. #5758

[static vars]: https://concourse-ci.org/vars.html#static-vars
[dynamic vars]: https://concourse-ci.org/vars.html#dynamic-vars

#### <sub><sup><a name="5782" href="#5782">:link:</a></sup></sub> fix

* The sidebar can now be expanded in the UI - no more long pipeline names being cut off! #5782