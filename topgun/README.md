# TOPGUN

This suite is one level above
[Testflight](https://github.com/concourse/testflight) in the sense that it will
target a BOSH deployment and make changes to the cluster. This is to test
things like workers disappearing, being recreated, etc.

## Test Boundaries

## Do

* Configure pipelines, run tasks, etc. - same as Testflight.
* Make changes to the BOSH deployment.
* Target the Garden/Baggageclaim APIs to verify container/volume presence.
* Mutate a VM (network partition, data corruption). These can happen in the
  real world. Just make sure these get cleaned up after.
* Pretend you're an operator - prefer `fly` over API use.

## Don't

* Modify the database directly. It's unrealistic to expect arbitrary SQL
  queries to have run against the database. Try to force scenarios using the
  above-defined surface area.
* Only use DB queries to verify state as a last resort, and replace these with
  `fly` commands once we bubble up enough information.
