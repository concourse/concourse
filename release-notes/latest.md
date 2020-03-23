This release bumps Concourse to v6.0 for good reason: it's the first time we've
changed how Concourse decides on the inputs for your jobs.

A whole new algorithm for deciding job inputs has been implemented which
performs much better for Concourse instances that have a ton of version and
build history. This algorithm works in a fundamentally different way, and in
some situations will yield different results than the previous algorithm. (More
details follow in the actual release notes below.)

In the past we've done major version bumps as a ceremony to celebrate big shiny
new features. This is the first time it's been done because there are
technically backwards-incompatible changes to fundamental pipeline semantics.

We have tested this release against larger scale than we ever tried to support
before, and deployed it to various environments of our own, and we've been
using it ourselves for a while now. Despite all that we still recommend that
anyone using Concourse for mission-critical workflows such as shipping security
updates should wait for the next few releases, *just in case* any edge cases
are found and fixed.

**IMPORTANT**: Please expect and prepare for some downtime when upgrading to
v6.0. On our large scale deployments we have observed 10-20 minutes of downtime
as the database is migrated, but this will obviously vary depending on the
amount of data.

#### <sub><sup><a name="3602" href="#3602">:link:</a></sup></sub> feature, breaking

* > **vito**: Hey Clara, want to write the release notes for the new algorithm?  \
  > **clarafu**: yeah sure whatever  \
  > **vito**: Try to spice it up a bit, it's not really a sexy feature.  \
  > **clarafu**: you got it boss

  Has this ever happened to you? "My Concourse is getting slower and slower
  even though I'm not adding any new pipelines!" "The web nodes are always
  under such heavy load!" "My database is constantly overloaded!"

  Well have no fear, because Algorithm v3 is here! #3602

  <p align="center">
    <img width="460" height="300" src="https://storage.googleapis.com/concourse-media-assets/wow.gif">
  </p>

  You might be wondering, what is the algorithm and why do I care about it? YOU
  FOOL! The algorithm is the heart and soul of Concourse! The algorithm is what
  determines the inputs for every newly created build in your pipeline.

  The main goals of the new algorithm is efficiency and correctness.

  The old algorithm used to load up all the resource versions, build inputs,
  and build outputs into memory then use brute-force to figure out what the
  next inputs would be. This method worked well enough in most cases, but with
  a long-lived deployment with thousands or even millions of versions or builds
  it would start to put a lot of strain on the `web` and `db` nodes just to
  load up the data set. This would theoretically get even worse when we change
  resources to [collect all versions][collect-all-versions-issue] as more
  versions would mean a larger dataset to process.

  The new algorithm takes a very different approach which does not require the
  entire dataset to be held in memory and cuts out nearly all of the "brute
  force" aspect of the old algorithm. We even make use of fancy [`jsonb`
  index][jsonb-index] functionality in Postgres; a successful build's set of
  resource versions are stored in a table which we can quickly search to find
  matching candidates when evaluating `passed` constraints.

  Overall, this new approach dramatically reduces resource utilization of both
  the `web` and `db` nodes. For a more detailed explanation of how the new
  algorithm works, check out the [section on this in the v10 blog
  post][v10-alg-update].

  Before we get into the shiny charts showing the improved performance, let's
  cover the breaking change that the new algorithm needed:

  [jsonb-index]: https://www.postgresql.org/docs/current/datatype-json.html#JSON-INDEXING
  [v10-alg-update]: https://blog.concourse-ci.org/core-roadmap-towards-v10/#issue-3602-a-new-algorithm
  [collect-all-versions-issue]: https://github.com/concourse/concourse/issues/5238

* **Breaking change:** for inputs with `passed` constraints, the algorithm now
  chooses versions based on the *build history* of each job in the `passed`
  constraint, rather than *version history* of the input's resource.

  This might make more sense with an example. Let's say we have a pipeline with
  a resource (`Resource`) that is used as an input to two jobs (`Job 1` and
  `Job 2`):

  ![Difference in behavior between old and new algorithm](https://storage.googleapis.com/concourse-media-assets/old-vs-new-algorithm-diagram.png)

  `Resource` has three versions: `v1` (oldest), `v2`, and `v3` (newest).

  `Job 1` has `Resource` as an unconstrained input, so it will always grab the
  latest version available - `v3`. In the scenario above, it has done this for
  `Build 1` but then a pipeline operator pinned `v1`, so `Build 2` then ran
  with `v1`. So now we have both `v1` and `v3` having "passed" `Job 1`, but in
  reverse order.

  The difference between the old algorithm and the new one is which version
  `Job 2` will use for its next build when `v1` is un-pinned.

  With the old algorithm, `Job 2` would choose `v3` as the input version as
  shown by the orange line. This is because the old algorithm would start from
  the *latest version* and then check if that version satisfies the `passed`
  constraints.

  With the new algorithm, `Job 2` will instead end up with `v1`, as shown by
  the green line. This is because the new algorithm starts with the versions
  from the *latest build* of the jobs listed in `passed` constraints, searching
  through older builds if necessary.

  The resulting behavior is that pipelines now flow versions downstream from
  job to job rather than requiring brute force. Jobs are now treated as the
  source of truth.

  This approach to selecting versions is much more efficient because it cuts
  out the "brute force" aspect: by treating the `passed` jobs as the source of
  versions, we *inherently* only attempt versions which already satisfy the
  constraints *and* passed through the same build together.

  The remaining challenge then is to find versions which satisfy *all* of the
  `passed` constraints, which the new algorithm does with a simple query
  utilizing a `jsonb` index to perform a sort of 'set intersection' at the
  database level. It's pretty neato!

* Now that the breaking change is out of the way, let's take a look at the
  metrics from our large-scale test environment and see if the whole thing was
  worth it from an efficiency standpoint.

  The first metric shows the database CPU utilization:

  ![Database CPU Usage](https://storage.googleapis.com/concourse-media-assets/new-vs-old-db-cpu.png)

  The left side shows that the CPU was completely pegged at 100% before the
  upgrade. This resulted in a slow web UI, slow pipeline scheduling
  performance, and complaints from our Concourse tenants.

  The right side shows that after upgrading to v6.0 the usage dropped to ~65%.
  This is still pretty high, but keep in mind that we intentionally gave this
  environment a pretty weak database machine so we don't just keep scaling up
  and pretending our users have unlimited funds for beefy hardware. Anything
  less than 100% usage here is a win.

  This next metric is shows database data transfer:

  ![Database Data Transfer](https://storage.googleapis.com/concourse-media-assets/new-vs-old-data-transfer.png)

  This shows that after upgrading to 6.0 we do a *lot* less data transfer from
  the database, because we no longer have to load the full algorithm dataset
  into memory.

  Not having to load the versions DB is also reflected in the amount of time it
  took just do do it as part of scheduling:

  ![Load VersionsDB machine hours](https://storage.googleapis.com/concourse-media-assets/algorithm-machine-hours.png)

  This graph shows that at the time just before the upgrade, the `web` node was
  spending 1 hour and 11 minutes of time per half-hour *just loading the
  dataset*. This entire step is gone, as reflected by the graph ending upon the
  upgrade to 6.0.

* You may be wondering how the upgrade's data migration works with such a huge
  change to how the algorithm deals with the data.

  The answer is: *very carefully*.

  If we were to do an in-place migration of all of the data to the new format
  used by the algorithm, the upgrade would take forever. To give you an idea of
  how long, even just adding a column to the `builds` table in our environment
  took about 16 minutes. Now imagine that multiplied by all of the inputs and
  outputs for each build.

  So instead of doing it all at once in a migration on startup, the algorithm
  will lazily migrate data for builds as it needs to. Overall, this should
  result in very little work to do as most jobs will have a satisfiable set of
  inputs without having to go too far back in the history of upstream jobs.

* Along with the new algorithm, we wanted to improve the transparency of
  showing why a build is pending and unable to determine its inputs. In the
  preparation view of a pending build, if the algorithm is failing to find an
  appropriate set of versions the UI will show the error message for the inputs
  that cannot be satisfied.

#### <sub><sup><a name="3832" href="#3832">:link:</a></sup></sub> fix

* The new algorithm fixes a case described in #3832. In this case, multiple
  resources with corresponding versions (e.g. a v1.2.3 semver resource and then
  a binary in S3 corresponding to that version) are correlated by virtue of
  being passed along the pipeline together.

  When one of the correlated versions was disabled, the old algorithm would
  incorrectly continue to use the other versions, matching it with an incorrect
  version for the resource whose version was disabled. Bad news bears!

  Because the new algorithm always works by selecting entire *sets* of versions
  at a time, they will always be correlated, and this problem goes away. Good
  news...uh, goats!

#### <sub><sup><a name="3704" href="#3704">:link:</a></sup></sub> feature, breaking

* LIDAR is now on by default! In fact, not only is it on by default, it is now
  THE ONLY OPTION. The old and busted 'Radar' resource checking component has
  been removed and the `--enable-lidar` flag will no longer be recognized.
  #3704

  With the switch to LIDAR, the metrics pertaining to resource checking have
  also changed (via #5171). Please consult the now-updated [Metrics
  documentation](https://concourse-ci.org/metrics.html) and update your
  dashboards accordingly!

#### <sub><sup><a name="413" href="#413">:link:</a></sup></sub> feature

* This next feature is one that has been asked for since the beginning of time. Build rerunning! #413 We finally did it, even though it is only the first iteration.

  You are finally able to rerun an old build with the same set of input versions without using the pinning trick. When you rerun a build, it will create a new build using the name of the original build with the rerun number appended to it. You can rerun a build either through the rerun button on the build page or through the fly command `fly rerun-build`. The rerun button is located at the top right corner of the build page, to the left of the button for manually triggering a new build.

  The rerun build will be ordered with the original build, so that it isn't treated as the "latest" build of the job (unless it is a rerun of the latest build) but rather as another run of the original build. This means that if you rerun an old build, the status of the build will not show up in the pipeline page for the status of the job. This is because the status of the job represents the current state of the job, which a rerun of an old build is not treated as the current state.

  Build rerunning fixes a key breaking change in the new algorithm. If you pin a resource to an old version and trigger a new build, once you unpin the resource there will be another new build scheduled using the latest version in order to respect the current state of the versions. This might be undesirable for the users that only wants to run a build using an old version without affecting the latest state of the builds. If you retrigger a build, it will only create a rerun of an old build and that does not affect the latest state of the job in regards to it's builds.

  This is currently a first pass at build retriggering, as it only supports retriggering builds that have the same set of inputs as the *current state of the job config*. What this means is that if you have an old build that only had two inputs and you have recently added a new input (so now you have a total of 3 inputs to this job), if you try to retrigger that old build that only used two inputs it will fail. This is because this first pass at build retriggering uses the latest state of the job config but runs it with the exact versions of the build that is being retriggered. That being said, there are future plans to have retriggering execute an exact rerun of a build including all the versions and job config that it used to run with originally. If you are interested in tracking the progress for the second pass at retriggering, the project epic is called [Build Lifecycle View](https://project.concourse-ci.org/projects/MDc6UHJvamVjdDM3NjI5MTk=).

#### <sub><sup><a name="4717" href="#4717">:link:</a></sup></sub> feature

* Along with the big changes to the algorithm, we also redesigned the build scheduler to hopefully help remove some unnecessary work. #4717 The old per pipeline scheduler is now transformed into one giant scheduler with one per ATC. This giant scheduler will now be responsible for scheduling all the jobs within the deployment.

  The exciting new feature is that now, it will only schedule jobs that "need to be scheduled". This means that if there is nothing happening, for example on the weekends when there are no new versions of resources or nobody triggering new builds, the scheduler will not run. This will help reduce unnecessary load to your web or database nodes. If you want to read more about what defines a job to "need to be scheduled", you can read the docs [here](_____) that describe exactly the cases that the scheduler will run for.

  As a small proof of the performance enhancement this feature adds, these are two metrics of the before and after of an upgrade to this new scheduling logic. There are two graphs, the one on the left labelled `Scheduling By Job` is a heat map that shows the time taken for each job to schedule. The graph on the right labelled `Total Time Scheduling Jobs + Loading Algorithm DB` shows the total time taken to schedule all jobs plus the total time taken to load the Algorithm database. Loading the algorithm database was used by the old algorithm in order to load up all the build inputs and outputs and resource versions into memory from the database. On the left side of both graphs, it shows the time taken for the old scheduler and on the right side it shows the time taken for the new scheduler.

  ![Old vs new Build Scheduling](https://concourse-ci.org/images/old-vs-new-build-scheduling.png)

  If we analyze the `Scheduling By Job` heap map, you will notice that the old scheduler consistently had a ton of jobs to schedule while the new scheduler has less jobs to schedule but might take longer to schedule each job. The new scheduler only schedules the jobs that need to be scheduled, which will result in less jobs scheduling overall but the time taken to schedule each job could possibly be slower than the old scheduler because of the difference in the new and old algorithm. The new algorithm needs to run small queries in order to find the next version for a build which is slower if you compare that to the old algorithm that already had all the versions loaded up into memory and only needs to number crunch. A contributing factor to this graph result is that it does not include the time it took to load up the versions DB for the old algorithm.

  Now if we look at the `Total Time Scheduling Jobs + Loading Algorithm DB` graph, we can see a big difference between the old and new scheduler. The total time the old scheduler took to schedule all the jobs plus the time it took to load up the versions DB is drastically higher than the total time taken to schedule all jobs by the new scheduler. This is because even though the time taken to schedule each job might be slightly slower in the new scheduler, because it no longer schedules all jobs every 10 seconds it results in a lot of time and CPU saved from doing unnecessary work.

  This is a new feature that is also risky in some ways. Because the "failure" case here would be that the scheduler does not run when it is expected to run and you would see no builds being scheduled. In order to help de-risk this failure case, we added a new fly command `fly schedule-job` that will kick off the scheduler if this ever happens.

#### <sub><sup><a name="4973" href="#4973">:link:</a></sup></sub> feature

* @evanchaoli introduced another new step type in #4973: the [`load_var` step](https://concourse-ci.org/steps.html#load-var-step)! This step can be used to load a value from a file at runtime and set it in a ["local var source"](https://concourse-ci.org/vars.html#local-vars) so that later steps in the build may pass the value to fields like `params`.

  With this primitive, resource type authors will no longer have to implement two ways to parameterize themselves (i.e. `tag` and `tag_file`). Resource types can now implement simpler interfaces which expect values to be set directly, and Concourse can handle the busywork of reading the value from a file.

  This feature, like `set_pipeline` step, is considered **experimental** until its corresponding RFC, [RFC #27](https://github.com/concourse/rfcs/pull/27) is resolved. The step will helpfully remind you of this fact by printing a warning on every single use.

#### <sub><sup><a name="4616" href="#4616">:link:</a></sup></sub> feature

* In #4614, @julia-pu implemented a way for `put` steps to automatically determine the artifacts they need, by configuring [`inputs: detect`](https://concourse-ci.org/steps.html#schema.step.put-step.inputs). With `detect`, the step will walk over its `params` and look for paths that correspond to artifact names in the build plan (e.g. `tag: foo/bar` or just `repository: foo`). When it comes time to run, only those named artifacts will be given to the step, which can avoid wasting a lot of time transferring artifacts the step doesn't even need.

  This feature may become the default in the future if it turns out to be useful and safe enough in practice. Try it out!

#### <sub><sup><a name="5118" href="#5118">:link:</a></sup></sub> feature

* #5118 implements infinite scroll and lazy rendering to the dashboard, which should greatly improve performance on installations with a ton of pipelines configured. The initial page load can still be quite laggy, but interacting with the page afterwards now performs a lot better. We'll keep chipping away at this problem and may have larger changes in store for the future.

#### <sub><sup><a name="5149" href="#5149">:link:</a></sup></sub> fix

* In #5149, @evanchaoli implemented an optimization which should lower the resource checking load on some instances: instead of checking *all* resources, only resources which are actually used as inputs will be checked.

#### <sub><sup><a name="5014" href="#5014">:link:</a></sup></sub> fix

* We fixed a bug where users that have upgraded from Concourse v5.6.0 to v5.8.0 with lidar enabled, they might experience a resource never being able to check because it is failing to create a check step. #5014

#### <sub><sup><a name="4065" href="#4065">:link:</a></sup></sub> fix

* Builds could get stuck in pending state for jobs that are set to run serially. If a build is scheduled but not yet started and the ATC restarts, the next time the build is picked up it will get stuck in pending state. This is because the ATC sees the job is set to run in serial and there is already a build being scheduled, so it will not continue to start that scheduled build. This bug is now fixed with the new release, where builds will never be stuck in a scheduled state because of it's serial configuration. #4065

#### <sub><sup><a name="5158" href="#5158">:link:</a></sup></sub> fix

* If you had lidar enabled, there is the possibility of some duplicate work being done in order to create checks for custom resource types. This happens when there are multiple resources that use the same custom resource type, they will all try to create a check for that custom type. In the end, there will only be one check that happens but the work involved with creating the check is duplicated. This bug was fixed so that there will be only one attempt to create a check for a custom resource type even if there are multiple resources that use it. #5158

#### <sub><sup><a name="5157" href="#5157">:link:</a></sup></sub> fix

* The length of time to keep around the history of a resource check was defaulted to 6 hours, but we discovered that this default might cause slowness for large deployments because of the number of checks that are kept around. The default is changed to 1 minute, and it is left up to the user to configure it higher if they would like to keep around the history of checks for longer. #5157

#### <sub><sup><a name="5023" href="#5023">:link:</a></sup></sub> fix

* The dashboard page refreshes its data every 5 seconds. Until now, it was possible (especially for admin users) for the dashboard to initiate an ever-growing number of API calls, unnecessarily consuming browser, network and API resources. Now the dashboard will not initiate a request for more data until the previous request finishes. #5023

#### <sub><sup><a name="4862" href="#4862">:link:</a></sup></sub> feature

* Whenever the dashboard page is loaded, it would decrypt and unmarshal all the job configs for all the teams that the user has access to. This would be slow if there are a ton of jobs. We made a change that would result in the dashboard no longer needing to decrypt or unmarshal the config of jobs, which will help speed up the loading of the dashboard page. #4862

#### <sub><sup><a name="4406" href="#4406">:link:</a></sup></sub> feature

* We have started adding a `--team` flag to Fly commands so that you can run them against different teams that you're authorized to perform actions against, without having to log in to the team with a separate Fly target. (#4406)

  So far, the flag has been added to `intercept`, `trigger-job`, `pause-job`, `unpause-job`, and `jobs`. In the future we will likely either continue with this change or start to re-think the overall Fly flow to see if there's a better alternative.

#### <sub><sup><a name="5075" href="#5075">:link:</a></sup></sub> fix

* Previously, the build tracker would unconditionally fire off a goroutine for each in-flight build (which then locks and short-circuits if the build is already tracked). We changed it so that the build tracker will only do so if we don't have a goroutine for it already. #5075

#### <sub><sup><a name="2724" href="#2724">:link:</a></sup></sub> fix

* We fixed a bug for job that have any type of serial groups set (`serial: true`, `serial_groups` or `max_in_flight`). Whenever a build for that job would be scheduled and we check for if the job has hit max in flight, it would unnecessarily recreate all the serial groups in the database. #2724

#### <sub><sup><a name="5039" href="#5039">:link:</a></sup></sub> fix

* The scheduler will separate the scheduling of rerun and regular builds (builds created by the scheduler and manually triggered builds) so that in situations where a rerun build is failing to schedule, maybe the input versions are not found, it will not block the scheduler from scheduling regular builds. #5039

#### <sub><sup><a name="4876" href="#4876">:link:</a></sup></sub> feature

* You can now easily enable or disable a resource version from the comfort of your command line using the new fly commands `fly enable-resource-version` and `fly disable-resource-version`, thanks to @stigtermichiel! #4876

#### <sub><sup><a name="5038" href="#5038">:link:</a></sup></sub> fix

* We fixed a bug where the existence of missing volumes that had child volumes referencing it was causing garbage collecting all missing volumes to fail. Missing volumes are any volumes that exists in the database but not on the worker. #5038

#### <sub><sup><a name="5100" href="#5100">:link:</a></sup></sub> fix

* The ResourceTypeCheckingInterval is not longer respected because of the removal of `radar` in this release with `lidar` becoming the default resource checker. Thanks to @evanchaoli for removed the unused flag `--resource-type-checking-interval`! #5100

#### <sub><sup><a name="4986" href="#4986">:link:</a></sup></sub> fix

* The link for the helm chart in the concourse github repo README was fixed thanks to @danielhelfand! #4986

#### <sub><sup><a name="4976" href="#4976">:link:</a></sup></sub> feature

* Include job label in build duration metrics exported to Prometheus. #4976

#### <sub><sup><a name="5093" href="#5093">:link:</a></sup></sub> fix

* The database will now use a version hash to look up resource caches in order to speed up any queries that reference resource caches. This will help speed up the resource caches garbage collection. #5093

#### <sub><sup><a name="5127" href="#5127">:link:</a></sup></sub> fix

* If you have `lidar` enabled, we fixed a bug where pinning an old version of a mock resource would cause it to become the latest version. #5127

#### <sub><sup><a name="5159" href="#5159">:link:</a></sup></sub> fix

* Explicitly whitelisted all traffic for concourse containers in order to allow outbound connections for containers on Windows. Thanks to @aemengo! #5159

#### <sub><sup><a name="5043" href="#5043">:link:</a></sup></sub> feature

* Add experimental support for exposing traces to [Jaeger] or [Stackdriver].

With this feature enabled (via `--tracing-(jaeger|stackdriver)-*` variables in
`concourse web`), the `web` node starts recording traces that represent the
various steps that a build goes through, sending them to the configured trace
collector. #5043

As this feature is being built using [OpenTelemetry], expect to have support for
other systems soon.

[OpenTelemetry]: https://opentelemetry.io/
[Jaeger]: https://www.jaegertracing.io/
[Stackdriver]: https://cloud.google.com/trace/

#### <sub><sup><a name="4092" href="#4092">:link:</a></sup></sub> feature

* @joshzarrabi added the `--all` flag to the `fly pause-pipeline` and
`fly unpause-pipeline` commands. This allows users to pause or unpause every
pipeline on a team at the same time. #4092

#### <sub><sup><a name="5133" href="#5133">:link:</a></sup></sub> fix

* In the case that a user has multiple roles on a team, the pills on the team headers on the dashboard now accurately reflect the logged-in user's most-privileged role on each team. #5133

#### <sub><sup><a name="5118" href="#5118">:link:</a></sup></sub> feature

* Improved the performance of the dashboard by only rendering the pipeline cards that are visible. #5118

#### <sub><sup><a name="4847" href="#4847">:link:</a></sup></sub> fix

* Set a default value of `4h` for `rebalance-interval`. Previously, this value was unset. With the new default, the workers will reconnect to a randomly selected TSA (SSH Gateway) every 4h.

#### <sub><sup><a name="5015" href="#5015">:link:</a></sup></sub> fix

* With #5015, worker state metrics will be emitted even for states with 0 workers, rather than not emitting the metric at all. This should make it easier to confirm that there are in fact 0 stalled workers as opposed to not having any knowledge of it.

#### <sub><sup><a name="5216" href="#5216">:link:</a></sup></sub> fix

* Bump golang.org/x/crypto module from `v0.0.0-20191119213627-4f8c1d86b1ba` to `v0.0.0-20200220183623-bac4c82f6975` to address vulnerability in ssh package.

#### <sub><sup><a name="5148" href="#5148">:link:</a></sup></sub> feature

* Improve the initial page load time by lazy-loading Javascript that isn't necessary for the first render. #5148

#### <sub><sup><a name="5262" href="#5262">:link:</a></sup></sub> feature

* Improve the dashboard load time by caching API responses to browser `localStorage` and first rendering a stale view of your pipelines. #5262

#### <sub><sup><a name="5113" href="#5113">:link:</a></sup></sub> feature

* @aledeganopix4d added a `last updated` column to the output of `fly pipelines` showing
the last date where the pipeline was set or reset. #5113

#### <sub><sup><a name="5275" href="#5275">:link:</a></sup></sub> fix

* Ensure the build page doesn't get reloaded when you highlight a log line, and fix auto-scrolling to a highlighted log line. #5275
