#### <sub><sup><a name="4950" href="#4950">:link:</a></sup></sub> feature, breaking

* "Have you tried logging out and logging back in?"
            - Probably every concourse operator at some point

  In the old login flow, concourse used to take all your upstream third party info (think github username, teams, etc) figure out what teams you're on, and encode those into your auth token. The problem with this approach is that every time you change your team config, you need to log out and log back in. 

  So now concourse doesn't do this anymore. Now we use a token directly from dex, the out-of-the-box identity provider that ships with concourse. If you're interested in the details you can check out [the issue](https://github.com/concourse/concourse/issues/2936).

  NOTE: this is a breaking change. You'll neeed to add a couple required flags at startup `CONCOURSE_CLIENT_SECRET` and `CONCOURSE_TSA_CLIENT_SECRET`. And, yes, you will need to log out and log back in one last time.

#### <sub><sup><a name="5305" href="#5305">:link:</a></sup></sub> feature

* We've updated the way that hijacked containers get garbage collected

  We are no longer relying on garden to clean up hijacked containers. Instead, we have implemented this functionality in concourse itself. This makes it much more portable to different container backends.

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

* Implented the core functionality for archiving pipelines [RFC #33]. 

  **note**: archived pipelines are neither visible in the web UI (#5370) nor in `fly pipelines`.

  **note:** archiving a pipeline will nullify the pipeline configuration. If for some reason you downgrade the version of Concourse, unpausing a pipeline that was previously archived will result in a broken pipeline. To fix that, set the pipeline again.

[RFC #33]: https://github.com/concourse/rfcs/pull/33

