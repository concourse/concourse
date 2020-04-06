#### <sub><sup><a name="4950" href="#4950">:link:</a></sup></sub> feature, breaking

* "Have you tried logging out and logging back in?"
            - Probably every concourse operator at some point

  In the old login flow, concourse used to take all your upstream third party info (think github username, teams, etc) figure out what teams you're on, and encode those into your auth token. The problem with this approach is that every time you change your team config, you need to log out and log back in. 

  So now concourse doesn't do this anymore. Now we use a token directly from dex, the out-of-the-box identity provider that ships with concourse. If you're interested in the details you can check out [the issue](https://github.com/concourse/concourse/issues/2936).

  NOTE: this is a breaking change. You'll neeed to add a couple required flags at startup `CONCOURSE_CLIENT_SECRET` and `CONCOURSE_TSA_CLIENT_SECRET`. And, yes, you will need to log out and log back in one last time.

#### <sub><sup><a name="5305" href="#5305">:link:</a></sup></sub> feature

* We've updated the way that hijacked containers get garbage collected

  We are no longer relying on garden to clean up hijacked containers. Instead, we have implemented this functionality in concourse itself. This makes it much more portable to different container backends.

#### <sub><sup><a name="5397" href="#5397">:link:</a></sup></sub> feature, breaking

* @pnsantos updated the Material Design icon library version to `5.0.45`.

  **note:** some icons changed names (e.g. `mdi-github-circle` was changed to `mdi-github`) so after this update you might have to update some `icon:` references
