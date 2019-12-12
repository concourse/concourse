#### <sub><sup><a name="4708" href="#4708">:link:</a></sup></sub> feature

* The first step (heh) along our [road to v10](https://blog.concourse-ci.org/core-roadmap-towards-v10/) has been taken!

  @evanchaoli implemented the [`set_pipeline` step](https://concourse-ci.org/set-pipeline-step.html) described by [RFC #31](https://github.com/concourse/rfcs/pull/31). The RFC is still technically in progress so the step is 'experimental' for now.

  The `set_pipeline` step allows a build to configure a pipeline within the build's team. This is the first "core" step type added since the concept of "build plans" was introduced, joining `get`, `put`, and `task`. Exciting!

  The key goal of the v10 roadmap is to support multi-branch and PR workflows, which require something more dynamic than `fly set-pipeline`. The theory is that by making pipelines more first-class - allowing them to be configured and automated by Concourse itself - we can support these more dynamic use cases by leveraging existing concepts instead of adding complexity to existing ones.

  As a refresher, here's where this piece fits in our roadmap for multi-branch/PR workflows:

  * With [RFC #33: archiving pipelines](https://github.com/concourse/rfcs/pull/33), any pipelines set by a `set_pipeline` step will be subject to automatic archival once a new build of the same job completes that no longer sets the pipeline. This way pipelines that are removed from the build plan will automatically go away, while preserving their build history.

  * With [RFC #34: instanced pipelines](https://github.com/concourse/rfcs/pull/34), pipelines sharing a common template can be configured with a common name, using `((vars))` to identify the instance. For example, you could have many instances of a `branches` pipeline, with `((branch_name))` as the "instance" var. Building on the previous point, instances which are no longer set by the build will be automatically archived.
  
  * With [RFC #29: spatial resources](https://github.com/concourse/rfcs/pull/29), the `set_pipeline` step can be automated to configure a pipeline instance corresponding to each "space" of a resource - i.e. all branches or pull requests in a repo. This RFC needs a bit of TLC (it hasn't been updated to be [prototype-based](https://blog.concourse-ci.org/reinventing-resource-types/)), but the basic idea is there.
  
  With all three of these RFCs delivered, we will have complete automation of pipelines for branches and pull requests! For more detail on the whole approach, check out the original [v10 blog post](https://blog.concourse-ci.org/core-roadmap-towards-v10/).
  
  Looking further ahead on the roadmap, [RFC #32: projects](https://github.com/concourse/rfcs/pull/32) proposes introduce a more explicit GitOps-style approach to configuration automation. In this context the `set_pipeline` step may feel a lot more natural. Until then, the `set_pipeline` step can be used as a simpler alternative to the [`concourse-pipeline` resource](https://github.com/concourse/concourse-pipeline-resource), with the key difference being that the `set_pipeline` step doesn't need any auth config.

#### <sub><sup><a name="4688" href="#4688">:link:</a></sup></sub> feature

* The pin menu on the pipeline page now matches the sidebar, and the dropdown toggles on clicking the pin icon #4688.

#### <sub><sup><a name="4556" href="#4556">:link:</a></sup></sub> feature

* Prometheus and NewRelic can receive Lidar check-finished event now #4556.

#### <sub><sup><a name="4707" href="#4707">:link:</a></sup></sub> feature

* Make Garden client HTTP timeout configurable. #4707

#### <sub><sup><a name="4698" href="#4698">:link:</a></sup></sub> feature

* @pivotal-bin-ju @taylorsilva @xtreme-sameer-vohra added batching to the NewRelic emitter and logging info for non 2xx responses from NewRelic #4698.

#### <sub><sup><a name="4748" href="#4748">:link:</a></sup></sub> feature

* @andhadley added support for Vault namespaces. #4748

#### <sub><sup><a name="4865" href="#4865">:link:</a></sup></sub> fix

* @kcmannem finally fixed the jagged edges on the progress bar indicators used by the dashboard. #4865

#### <sub><sup><a name="4749" href="#4749">:link:</a></sup></sub> fix

* @evanchaoli fixed a weird behavior with secret redaction wherein a secret containing e.g. `{` on its own line (i.e. formatted JSON) would result in `{` being replaced with `((redacted))` in build logs. Single-character lines will instead be skipped.

  As an aside, anyone with a truly single-character credential *may* want to add another character or two.

#### <sub><sup><a name="4804" href="#4804">:link:</a></sup></sub> fix

* @vito bumped the `autocert` dependency so that Let's Encrypt will default to the ACME v2 API. #4804

#### <sub><sup><a name="registry-image-0.8.2" href="#registry-image-0.8.2">:link:</a></sup></sub> fix

* Bumped the [`registry-image` resource](https://github.com/concourse/registry-image-resource) to [v0.8.2](https://github.com/concourse/registry-image-resource/releases/tag/v0.8.2), which should resolve `DIGEST_INVALID` errors (among others) introduced by faulty retry logic. Additionally, the resource will now retry on `429 Too Many Requests` errors from the registry, with exponential back-off up to 1 hour.

#### <sub><sup><a name="4808" href="#4808">:link:</a></sup></sub> fix

* @evanchaoli fixed a race condition resulting in a crash with LIDAR enabled. #4808

#### <sub><sup><a name="4817" href="#4817">:link:</a></sup></sub> fix

* @evanchaoli fixed a regression introduced with the secret redaction work which resulted in build logs being buffered. #4817

#### <sub><sup><a name="4746" href="#4746">:link:</a></sup></sub> fix

* Fixed the problem of when fail_fast for in_parallel is true, a failing step causes the in_parallel to fall into on_error #4746

#### <sub><sup><a name="4816" href="#4816">:link:</a></sup></sub> fix

* @witjem removed superfluous mentions of `register-worker` from TSA. #4816
