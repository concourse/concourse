---
name: 🛩 Ship a patch release
about: Create an issue for tracking a patch release.
title: 'Release X.X.X'
assignees: ''

---

Steps for a new patch release:

* [ ] Ensure each resource type is pinned to whatever version was last shipped within the MAJOR.MINOR series. This is to avoid accidentally shipping breaking changes in resource types with patch releases.

  * If a patch release is being shipped in order to bump a resource type (e.g. for a CVE or bug fix), pin it to the appropriate version instead.

* [ ] Go through all the `needs-documentation` PRs in the release page for your milestone `https://project.concourse-ci.org/releases/concourse?milestone=v<M.m.p>` and make sure that everything has proper documentation within `concourse/docs` (if needed). You can organize which PRs by clicking on the button to add whichever label best fits that PR.

  * If it is already documented within `concourse/docs`, add a `release/documented` label
  * If there is no documentation and the changes have user impact that should be documented, add the documentation to `concourse/docs`(or delegate) then add a `release/documented` label after finished. E.g. the addition of a new step type ( set_pipeline step).
  * If there is no documentation and the changes have user impact that do not need to be documented, add a `release/undocumented` label. E.g. an experimental feature.
  * If there is no documentation and the changes do not have user impact, add a `release/no-impact` label. E.g. refactors.
  
* [ ] Once the final commit has made it through the pipeline, the `create-draft-release` job can be triggered. This job will create a draft release within the concourse GitHub [release page](https://github.com/concourse/concourse/releases) where you can make any final adjustments or arrangements to the generated release notes. **PLEASE NOTE that any manual changes made on the draft release WILL BE OVERWRITTEN if you retrigger the `create-draft-release` job**. Please be sure to only make manual edits AFTER you are sure this is the final run of the job.

  * If you would like to edit the content, you can directly edit the PRs that it was generated from. The title is used for each PR and also the body within the `Release Note` header in the PR. After you have made your edits within the PR, you can rerun the `create-draft-release` job in order to regenerate a new release note.

  * If you would like to change the arrangement of the PRs within the release note, you can make the edits directly on the release note of the draft release. 

* [ ] Once everything is ready, the `shipit` job can be triggered. The `publish-binaries` job will convert your draft release into a final release including the body of your draft release (which will persist any changes you made to the draft release body). Subsequently, the [promote concourse job](https://ci.concourse-ci.org/teams/main/pipelines/promote) will run automatically. The `publish-docs` job runs automatically, *as long as the version is actually the latest version available*.

* [ ] The [helm-chart pipeline](https://ci.concourse-ci.org/teams/main/pipelines/helm-chart?group=dependencies&group=publish) is used to bump & then publish the chart.

  * Merge the `release/` branch into `master`.
  * Next, run the `concourse-app-bump` job (bumps the app version and image to point to the latest release)
  * Finally, run the `publish-chart-{major|minor|patch}` job, depending on what has changed in the chart
  * If you make a major bump, be sure to update the `CHANGELOG.md` in the concourse-chart repo
