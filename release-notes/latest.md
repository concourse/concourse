#### <sub><sup><a name="5429" href="#5429">:link:</a></sup></sub> feature

* Operators can now limit the number of concurrent API requests that your web node will serve by passing a flag like `--concurrent-request-limit action:limit` where `action` is the API action name as they appear in the [action matrix in our docs](https://concourse-ci.org/user-roles.html#action-matrix).

  If the web node is already concurrently serving the maximum number of requests allowed by the specified limit, any additional concurrent requests will be rejected with a `503 Service Unavailable` status. If the limit is set to `0`, the endpoint is effectively disabled, and all requests will be rejected with a `501 Not Implemented` status.

Currently the only API action that can be limited in this way is `ListAllJobs` -- we considered allowing this limit on arbitrary endpoints but didn't want to enable operators to shoot themselves in the foot by limiting important internal endpoints like worker registration.

  It is important to note that, if you use this configuration, it is possible for super-admins to effectively deny service to non-super-admins. This is because when super-admins look at the dashboard, the API returns a huge amount of data (much more than the average user) and it can take a long time (over 30s on some clusters) to serve the request. If you have multiple super-admin dashboards open, they are pretty much constantly consuming some portion of the number of concurrent requests your web node will allow. Any other requests, even if they are potentially cheaper for the API to service, are much more likely to be rejected because the server is overloaded by super-admins. Still, the web node will no longer crash in these scenarios, and non-super-admins will still see their dashboards, albeit without nice previews. To work around this scenario, it is important to be careful of the number of super-admin users with open dashboards. #5429

#### <sub><sup><a name="remove-disable-list-all-jobs" href="#remove-disable-list-all-jobs">:link:</a></sup></sub> breaking

* The above-mentioned `--concurrent-request-limit` flag replaces the `--disable-list-all-jobs` flag introduced in [v5.2.8](https://github.com/concourse/concourse/releases/tag/v5.2.8) and [v5.5.9](https://github.com/concourse/concourse/releases/tag/v5.5.9#5340). To get consistent functionality, change `--disable-list-all-jobs` to `--concurrent-request-limit ListAllJobs:0` in your configuration. #5429

#### <sub><sup><a name="strict-env-vars" href="#strict-env-vars">:link:</a></sup></sub> breaking

* It has long been possible to configure concourse either by passing flags to the binary, or by passing their equivalent `CONCOURSE_*` environment variables. Until now we had noticed that when an environment variable is passed, the flags library we use would treat it as a "default" value -- this is a [bug](https://github.com/jessevdk/go-flags/issues/329). We issued a PR to that library adding stricter validation for flags passed via environment variables. What this means is that operators may have been passing invalid configuration via environment variables and concourse wasn't complaining -- after this upgrade, that invalid configuration will cause the binary to fail. Hopefully it's a good prompt to fix up your manifests! #5429

#### <sub><sup><a name="5057" href="#5057">:link:</a></sup></sub> feature
 
* @HannesHasselbring and @tenjaa added a prometheus metric of the amount of tasks that are currently waiting to be scheduled when using the `limit-active-tasks` placement strategy
