#### <sub><sup><a name="5429" href="#5429">:link:</a></sup></sub> feature

* Operators can now limit the number of concurrent API requests that your web node will serve by passing a flag like `--concurrent-request-limit action:limit` where `action` is the API action name as they appear in the [action matrix in our docs](https://concourse-ci.org/user-roles.html#action-matrix).

  If the web node is already concurrently serving the maximum number of requests allowed by the specified limit, any additional concurrent requests will be rejected with a `503 Service Unavailable` status. If the limit is set to `0`, the endpoint is effectively disabled, and all requests will be rejected with a `501 Not Implemented` status.

Currently the only API action that can be limited in this way is `ListAllJobs` -- we considered allowing this limit on arbitrary endpoints but didn't want to enable operators to shoot themselves in the foot by limiting important internal endpoints like worker registration. If the `ListAllJobs` endpoint is disabled completely (with a concurrent request limit of 0), the dashboard reflects this by showing empty pipeline cards labeled 'no data'.

  It is important to note that, if you use this configuration, it is possible for super-admins to effectively deny service to non-super-admins. This is because when super-admins look at the dashboard, the API returns a huge amount of data (much more than the average user) and it can take a long time (over 30s on some clusters) to serve the request. If you have multiple super-admin dashboards open, they are pretty much constantly consuming some portion of the number of concurrent requests your web node will allow. Any other requests, even if they are potentially cheaper for the API to service, are much more likely to be rejected because the server is overloaded by super-admins. Still, the web node will no longer crash in these scenarios, and non-super-admins will still see their dashboards, albeit without nice previews. To work around this scenario, it is important to be careful of the number of super-admin users with open dashboards. #5429, #5529

#### <sub><sup><a name="remove-disable-list-all-jobs" href="#remove-disable-list-all-jobs">:link:</a></sup></sub> breaking

* The above-mentioned `--concurrent-request-limit` flag replaces the `--disable-list-all-jobs` flag introduced in [v5.2.8](https://github.com/concourse/concourse/releases/tag/v5.2.8) and [v5.5.9](https://github.com/concourse/concourse/releases/tag/v5.5.9#5340). To get consistent functionality, change `--disable-list-all-jobs` to `--concurrent-request-limit ListAllJobs:0` in your configuration. #5429

#### <sub><sup><a name="strict-env-vars" href="#strict-env-vars">:link:</a></sup></sub> breaking

* It has long been possible to configure concourse either by passing flags to the binary, or by passing their equivalent `CONCOURSE_*` environment variables. Until now we had noticed that when an environment variable is passed, the flags library we use would treat it as a "default" value -- this is a [bug](https://github.com/jessevdk/go-flags/issues/329). We issued a PR to that library adding stricter validation for flags passed via environment variables. What this means is that operators may have been passing invalid configuration via environment variables and concourse wasn't complaining -- after this upgrade, that invalid configuration will cause the binary to fail. Hopefully it's a good prompt to fix up your manifests! #5429

#### <sub><sup><a name="5057" href="#5057">:link:</a></sup></sub> feature
 
* @shyamz-22, @HannesHasselbring and @tenjaa added a metric for the amount of tasks that are currently waiting to be scheduled when using the `limit-active-tasks` placement strategy. #5448

#### <sub><sup><a name="5082" href="#5082">:link:</a></sup></sub> fix

* Close Worker's registration connection to the TSA on application level keepalive failure
* Add 5 second timeout for keepalive operation. #5802

#### <sub><sup><a name="5457" href="#5457">:link:</a></sup></sub> fix

* Improve consistency of auto-scrolling to highlighted logs. #5457

#### <sub><sup><a name="5452" href="#4081">:link:</a></sup></sub> fix

* @shyamz-22 added ability to configure NewRelic insights endpoint which allows us to use EU or US data centers. #5452

#### <sub><sup><a name="5520" href="#5520">:link:</a></sup></sub> fix

* Fix a bug that when `--log-db-queries` is enabled only part of DB queries were logged. Expect to see more log outputs when using the flag now. #5520

#### <sub><sup><a name="5485" href="#5485">:link:</a></sup></sub> fix

* Fix a bug where a Task's image or input volume(s) were redundantly streamed from another worker despite having a local copy. This would only occur if the image or input(s) were provided by a resource definition (eg. Get step). #5485

#### <sub><sup><a name="5604" href="#5604">:link:</a></sup></sub> fix

* Previously, aborting a build could sometimes result in an `errored` status rather than an `aborted` status. This happened when step code wrapped the `err` return value, fooling our `==` check. We now use [`errors.Is`](https://golang.org/pkg/errors/#Is) (new in Go 1.13) to check for the error indicating the build has been aborted, so now the build should be correctly given the `aborted` status even if the step wraps the error. #5604

#### <sub><sup><a name="5596" href="#5595">:link:</a></sup></sub> fix
 
* @lbenedix and @shyamz-22 improved the way auth config for teams are validated. Now operators cannot start a web node with an empty `--main-team-config` file, and `fly set-team` will fail if it would result in a team with no possible members. This prevents scenarios where users can get [accidentally locked out](https://github.com/concourse/concourse/issues/5595) of concourse. #5596

#### <sub><sup><a name="5013" href="#5013">:link:</a></sup></sub> feature

* Support path templating for secret lookups in Vault credential manager.

  Previously, pipeline and team secrets would always be searched for under "/prefix/TEAM/PIPELINE/" or "/prefix/TEAM/", where you could customize the prefix but nothing else. Now you can supply your own templates if your secret collections are organized differently, including for use in `var_sources`. #5013

#### <sub><sup><a name="5622" href="#5622">:link:</a></sup></sub> fix

* @evanchaoli enhanced to change the Web UI and `fly teams` to show teams ordering by team names, which allows users who are participated in many teams to find a specific team easily. #5622

#### <sub><sup><a name="5639" href="#5639">:link:</a></sup></sub> fix

* Fix a bug that crashes web node when renaming a job with `old_name` equal to `name`. #5639

#### <sub><sup><a name="5620" href="#5620">:link:</a></sup></sub> fix

* @evanchaoli enhanced task step `vars` to support interpolation. #5620

#### <sub><sup><a name="5624" href="#5624">:link:</a></sup></sub> fix

* Fixed a bug where fly would no longer tell you if the team you logged in with was invalid. #5624

#### <sub><sup><a name="5192" href="#5192">:link:</a></sup></sub> fix

* @evanchaoli changed the behaviour of the web to retry individual build steps that fail when a worker disappears. #5192

#### <sub><sup><a name="5549" href="#5549">:link:</a></sup></sub> fix

* Added a new HTTP wrapper that returns HTTP 409 for endpoints listed in concourse/rfc#33 when the requested pipeine is archived. #5549

#### <sub><sup><a name="5576" href="#5576">:link:</a></sup></sub> fix

* @dcsg added support for AWS SSM for `var_sources`. #5576

#### <sub><sup><a name="5575" href="#5575">:link:</a></sup></sub> feature

* Added tracing to the lidar component, where a single trace will be emitted for each run of the scanner and the consequential checking that happens from the checker. The traces will allow for more in depth monitoring of resource checking through describing how long each resource is taking to scan and check. #5575

#### <sub><sup><a name="5617" href="#5617">:link:</a></sup></sub> fix

* @ozzywalsh added the `--team` flag to the `fly unpause-pipeline` command. #5617
