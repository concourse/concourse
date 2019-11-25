#### <sub><sup><a name="4797" href="#4797">:link:</a></sup></sub> fix, security

* *Attention CF auth users*

  We recently discovered that cloud foundry org (and space) names can contain
  all sorts of special characters. This is a problem because the cf auth
  connector uses a `:` as a delimiter. When a user logs in using the cf auth
  connector we get back a token with a `groups` claim, where groups are of the
  form `CONNECTOR:ORG` and `CONNECTOR:ORG:SPACE`. This means that an org
  `myorg` with space `myspace` would be returned as `cf:myorg:myspace` which is
  equivalent to an org named `myorg:myspace`.

  *If a malicious user has the ability to create orgs or spaces within their
  cloud foundry deployment, they would be able to use this fact to gain access
  to unauthorized teams in concourse.*
  
  As a result, we have introduced a way to map org and space `guids` to teams
  in concourse, and now discourage the use of referencing orgs and spaces by
  `name`. In order to emphasize this behaviour we have made a breaking change
  to the cf auth team flags, used during `fly set-team` or as `env` variables
  when configuring the main team at startup.  These affected flags are now
  prefixed with an `insecure` label.

  If you configure the `main` team with cf auth during startup, your concourse
  may fail to start after migrating to this version. You will need to either
  use these new `insecure` flags, or use org and space guids.

  Any teams using cf auth that have been configure with `fly set-team` will
  continue to work as before however the next time you update your team config,
  you will need to use the new flags.

#### <sub><sup><a name="4712" href="#4712">:link:</a></sup></sub> feature

* In addition to fixing the above issue, CF auth users also now have the
  ability to grant team access based on specific user roles in CloudFoundry.

  Previously, you could only grant access to users having the 'developer' role.

#### <sub><sup><a name="4688" href="#4688">:link:</a></sup></sub> feature

* The pin menu on the pipeline page now matches the sidebar, and the dropdown
  toggles on clicking the pin icon #4688.

#### <sub><sup><a name="4556" href="#4556">:link:</a></sup></sub> feature

* Prometheus and NewRelic can receive Lidar check-finished event now #4556.

#### <sub><sup><a name="4707" href="#4707">:link:</a></sup></sub> feature

* Make Garden client HTTP timeout configurable. #4707

#### <sub><sup><a name="4698" href="#4698">:link:</a></sup></sub> feature

* @pivotal-bin-ju @taylorsilva @xtreme-sameer-vohra added batching to the
  NewRelic emitter and logging info for non 2xx responses from NewRelic #4698.
