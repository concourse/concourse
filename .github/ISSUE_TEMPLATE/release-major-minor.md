---
name: 🚀 Ship a major or minor release
about: Create an issue for tracking a major or minor release.
title: 'Release X.X.X'
assignees: ''

---

Steps for a new major/minor release:

* [ ] Add this issue to the `v<M.m.x>` milestone

* [ ] Create your release branch on the `concourse/concourse` github repo with the following format `release/M.m.x` (M being the major version and m being the minor version)

* [ ] Create the release branch on `concourse/concourse-docker` repository.

* [ ] Create the release branch on the `concourse/concourse-bosh-release` repository. Make any missing changes to the spec of `web` or `worker` depending on if the release contains any changes that adds or modifies any flags.
  * Any changes you make on the branch will not get automatically merged back to master so try to make the changes on master and then create the branch from there.
  * We should really have something that will merge the branch back to master. (like we do for the `concourse/concourse` branches)

* [ ] Create the release branch on `concourse/concourse-bosh-deployment` repository.

* [ ] Create the release branch on `concourse/concourse-chart` repository from the `dev` branch. Make any missing changes to `values.yaml` or `templates/web-deployment.yaml` for changes to flags on web or `templates/worker-deployment.yaml` for changes to flags on the worker.

* [ ] Bump the appropriate versions for resource types. Take a look at changes made since the last release of each resource, see what they entail and bump the version in their respective pipeline in `ci.concourse-ci.org`.

  * If the changes were only README or repo restructuring changes with no user impact, you don't need to bump the version
  * If the changes were small bug fixes or changes, you can do a patch version bump
  * If the changes were adding of features, you can do a minor version bump
  * If the changes involve a breaking changes, that should be a major version bump
  * Resource Types:
    * [ ] https://github.com/concourse/git-resource/
    * [ ] https://github.com/concourse/registry-image-resource
    * [ ] https://github.com/concourse/time-resource/
    * [ ] https://github.com/concourse/s3-resource
    * [ ] https://github.com/concourse/github-release-resource/
    * [ ] https://github.com/concourse/pool-resource/
    * [ ] https://github.com/concourse/semver-resource
    * [ ] https://github.com/concourse/docker-image-resource/
    * [ ] https://github.com/concourse/mock-resource/
    * [ ] https://github.com/concourse/hg-resource
    * [ ] https://github.com/concourse/bosh-io-release-resource
    * [ ] https://github.com/concourse/bosh-io-stemcell-resource

* [ ] Add your release pipeline to the `reconfigure-pipeline`

* [ ] Once the all source code changes are finalized, Concourse RC version should be deployed to CI using [dispatcher.concourse-ci.org](https://dispatcher.concourse-ci.org/)
  * including all the external workers (Currently only the `bosh` tagged worker deployed in [`concourse/prod`](https://github.com/concourse/prod))

* [ ] If you are doing a major release (or a release that involves a risky large feature), consider creating a [drills environment](https://github.com/concourse/drills) for some stress testing to ensure that the release does not involve any performance regressions.

* [ ] Once the final commit has made it through the pipeline, the `create-draft-release` job can be triggered. This job will create a draft release within the concourse GitHub [release page](https://github.com/concourse/concourse/releases) where you can make any final adjustments or arrangements to the generated release notes. **PLEASE NOTE that any manual changes made on the draft release WILL BE OVERWRITTEN if you retrigger the `create-draft-release` job**. Please be sure to only make manual edits AFTER you are sure this is the final run of the job.
  * If you would like to edit the content, you can directly edit the PRs that it was generated from. The title is used for each PR and also the body within the `Release Note` header in the PR. After you have made your edits within the PR, you can rerun the `create-draft-release` job in order to regenerate a new release note.
  * If you would like to change the arrangement of the PRs within the release note, you can make the edits directly on the release note of the draft release. 

* [ ] Once everything is ready, the `shipit` job can be triggered. The `publish-binaries` job will convert your draft release into a final release including the body of your draft release (which will persist any changes you made to the draft release body). Subsequently, the [promote concourse job](https://ci.concourse-ci.org/teams/main/pipelines/promote) will run automatically. The `publish-docs` job runs automatically.

* [ ] Release a new version of the Helm chart
  * Merge the `release/` branch into `master` by making a PR
  * Update the `helm-chart` pipeline to point to the `release/` branch
  * Next, run the `concourse-app-bump` job (bumps the app version and image to point to the latest release): https://ci.concourse-ci.org/teams/main/pipelines/helm-chart?vars.version=7&group=dependencies
  * Run the `k8s-smoke` job
  * Finally, run the `publish-chart-{major|minor|patch}` job, depending on what has changed in the chart
  * If you make a major bump, be sure to update the `CHANGELOG.md` in the concourse-chart repo

* [ ] Once the Concourse release is shipped, the final version should be deployed to production
