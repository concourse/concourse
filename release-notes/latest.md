#### <sub><sup><a name="5833" href="#5833">:link:</a></sup></sub> feature

* Added a way of renaming pipeline resources while preserving version history by updating the resource name (as well as any reference in steps) and specifying its old name as [**`old_name`**](https://concourse-ci.org/resources.html#schema.resource.old_name). After the pipeline has been configured, the `old_name` field can be removed. #5833

#### <sub><sup><a name="5777" href="#5777">:link:</a></sup></sub> feature

* Archived pipelines can be displayed in the web UI via a toggle switch in the top bar. #5777, #5760

#### <sub><sup><a name="5780" href="#5780">:link:</a></sup></sub> fix

* Fixed a regression where builds could be stuck pending forever if an input was pinned by partially specifying a version via the [`version:` on a `get` step](https://concourse-ci.org/jobs.html#schema.step.get-step.version), [`version:` on a resource config](https://concourse-ci.org/resources.html#schema.resource.version) or by running [`fly pin-resource`](https://concourse-ci.org/managing-resources.html#fly-pin-resource). #5780

#### <sub><sup><a name="5758" href="#5758">:link:</a></sup></sub> fix

* @evanchaoli fixed a regression that prevented using both [static vars] and [dynamic vars] simultaneously in a task. #5758

[static vars]: https://concourse-ci.org/vars.html#static-vars
[dynamic vars]: https://concourse-ci.org/vars.html#dynamic-vars

#### <sub><sup><a name="5821" href="#5821">:link:</a></sup></sub> fix

* Pipelines can be re-ordered in the dashboard when filtering. This was a regression introduced in 6.0. #5821

#### <sub><sup><a name="5778" href="#5778">:link:</a></sup></sub> feature

* Style improvements to the sidebar. #5778

#### <sub><sup><a name="5782" href="#5782">:link:</a></sup></sub> fix

* The sidebar can now be expanded in the UI - no more long pipeline names being cut off! #5782

#### <sub><sup><a name="5390" href="#5390">:link:</a></sup></sub> feature

* Add `--include-archived` flag for `fly pipelines` command. #5673

#### <sub><sup><a name="5770" href="#5770">:link:</a></sup></sub> fix

* `fly login` now accepts arbitrarily long tokens when pasting the token manually into the console. Previously, the limit was OS dependent (with OSX having a relatively small maximum length of 1024 characters). This has been a long-standing issue, but it became most noticable after 6.1.0 which significantly increased the size of tokens. Note that pasted token is now hidden in the console output. #5770

#### <sub><sup><a name="5390" href="#5390">:link:</a></sup></sub> feature

* Add `--team` flag for `fly set-pipelines` command #5805
