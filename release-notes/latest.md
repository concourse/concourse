### <sub><sup><a name="4950" href="#4950">:link:</a></sup></sub> feature, breaking

* "Have you tried logging out and logging back in?"
            - Probably every concourse operator at some point

  In the old login flow, concourse used to take all your upstream third party info (think github username, teams, etc) figure out what teams you're on, and encode those into your auth token. The problem with this approach is that every time you change your team config, you need to log out and log back in. So now concourse doesn't do this anymore. Instead we use a token directly from dex, the out-of-the-box identity provider that ships with concourse.

  This new flow does introduce a few additional database calls on each request, but we've added some mitigations (caching and batching) to help reduce the impact. If you're interested in the details you can check out [the original issue](https://github.com/concourse/concourse/issues/2936) or the follow up with some of the [optimizations](https://github.com/concourse/concourse/pull/5257).

  NOTE: And yes, you will need to log out and log back in after upgrading.

#### <sub><sup><a name="5429" href="#5429">:link:</a></sup></sub> feature

* Operators can now limit the number of concurrent API requests that your web node will serve by passing a flag like `--concurrent-request-limit action:limit` where `action` is the API action name as they appear in the [action matrix in our docs](https://concourse-ci.org/user-roles.html#action-matrix).

  If the web node is already concurrently serving the maximum number of requests allowed by the specified limit, any additional concurrent requests will be rejected with a `503 Service Unavailable` status. If the limit is set to `0`, the endpoint is effectively disabled, and all requests will be rejected with a `501 Not Implemented` status.

Currently the only API action that can be limited in this way is `ListAllJobs` -- we considered allowing this limit on arbitrary endpoints but didn't want to enable operators to shoot themselves in the foot by limiting important internal endpoints like worker registration.

  It is important to note that, if you use this configuration, it is possible for super-admins to effectively deny service to non-super-admins. This is because when super-admins look at the dashboard, the API returns a huge amount of data (much more than the average user) and it can take a long time (over 30s on some clusters) to serve the request. If you have multiple super-admin dashboards open, they are pretty much constantly consuming some portion of the number of concurrent requests your web node will allow. Any other requetss, even if they are potentially cheaper for the API to service, are much more likely to be rejected because the server is overloaded by super-admins. Still, the web node will no longer crash in these scenarios, and non-super-admins will still see their dashboards, albeit without nice previews. To work around this scenario, it is important to be careful of the number of super-admin users with open dashboards. #5429

#### <sub><sup><a name="remove-disable-list-all-jobs" href="#remove-disable-list-all-jobs">:link:</a></sup></sub> breaking

* The above-mentioned `--concurrent-request-limit` flag replaces the `--disable-list-all-jobs` flag introduced in [v5.2.8](https://github.com/concourse/concourse/releases/tag/v5.2.8) and [v5.5.9](https://github.com/concourse/concourse/releases/tag/v5.5.9#5340). To get consistent functionality, change `--disable-list-all-jobs` to `--concurrent-request-limit ListAllJobs:0` in your configuration. #5429

#### <sub><sup><a name="strict-env-vars" href="#strict-env-vars">:link:</a></sup></sub> breaking

* It has long been possible to configure concourse either by passing flags to the binary, or by passing their equivalent `CONCOURSE_*` environment variables. Until now we had noticed that when an environment variable is passed, the flags library we use would treat it as a "default" value -- this is a [bug](https://github.com/jessevdk/go-flags/issues/329). We issued a PR to that library adding stricter validation for flags passed via environment variables. What this means is that operators may have been passing invalid configuration via environment variables and concourse wasn't complaining -- after this upgrade, that invalid configuration will cause the binary to fail. Hopefully it's a good prompt to fix up your manifests! #5429

#### <sub><sup><a name="5305" href="#5305">:link:</a></sup></sub> feature

* We've updated the way that hijacked containers get garbage collected

  We are no longer relying on garden to clean up hijacked containers. Instead, we have implemented this functionality in concourse itself. This makes it much more portable to different container backends.

##### <sub><sup><a name="5431" href="#5431">:link:</a></sup></sub> feature

* We've updated the way that containers associated with failed runs get garbage collected

  Containers associated with failed runs used to sit around until a new run is executed.  They now have a max lifetime (default - 120 hours), configurable via 'failed-grace-period' flag.

#### <sub><sup><a name="5375" href="#5375">:link:</a></sup></sub> fix

* Fix rendering pipeline previews on the dashboard on Safari. #5375

#### <sub><sup><a name="5377" href="#5377">:link:</a></sup></sub> fix

* Fix pipeline tooltips being hidden behind other cards. #5377

#### <sub><sup><a name="5384" href="#5384">:link:</a></sup></sub> fix

* Fix log highlighting on the one-off-build page. Previously, highlighting any log lines would cause the page to reload. #5384

#### <sub><sup><a name="5392" href="#5392">:link:</a></sup></sub> fix

* Fix regression which inhibited scrolling through the build history list. #5392

#### <sub><sup><a name="5397" href="#5397">:link:</a></sup></sub> feature, breaking

* @pnsantos updated the Material Design icon library version to `5.0.45`.

  **note:** some icons changed names (e.g. `mdi-github-circle` was changed to `mdi-github`) so after this update you might have to update some `icon:` references

#### <sub><sup><a name="5410" href="#5410">:link:</a></sup></sub> feature

* We've moved the "pin comment" field in the Resource view to the top of the page (next to the currently pinned version). The comment can be edited inline.

#### <sub><sup><a name="5368" href="#5368">:link:</a></sup></sub> feature

* Implemented the core functionality for archiving pipelines [RFC #33]. 

  **note**: archived pipelines are neither visible in the web UI (#5370) nor in `fly pipelines`.

  **note:** archiving a pipeline will nullify the pipeline configuration. If for some reason you downgrade the version of Concourse, unpausing a pipeline that was previously archived will result in a broken pipeline. To fix that, set the pipeline again.

[RFC #33]: https://github.com/concourse/rfcs/pull/33

#### <sub><sup><a name="5458" href="#5458">:link:</a></sup></sub> feature

* Add loading indicator on dashboard while awaiting initial API/cache response. #5458

#### <sub><sup><a name="5496" href="#5496">:link:</a></sup></sub> fix

* Allow the dashboard to recover from the "show turbulence" view if any API call fails once, but starts working afterward. This will prevent users from needing to refresh the page after closing their laptop or in the presence of network flakiness. #5496

#### <sub><sup><a name="5479" href="#5479">:link:</a></sup></sub> feature

* Updated a migration that adds a column to the pipelines table. The syntax initially used is not supported by Postgres 9.5 which is still supported. Removed the unsupported syntax so users using Postgres 9.5 can run the migration. Our CI pipeline has also been updated to ensure we run our tests on Postgres 9.5. #5479

#### <sub><sup><a name="5452" href="#5452">:link:</a></sup></sub> fix

* We fixed a bug where if you create a new build and then trigger a rerun build, both the builds will be stuck in pending state. #5452
