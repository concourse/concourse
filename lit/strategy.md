Getting Started
  * Installing `fly`
    * sync: Update your local copy of fly
  * Targeting & Logging In
    * login: Authenticating with and saving Concourse targets
    * logout: Remove authentication and delete Concourse targets
    * targets: List the current targets

Tasks
  * Schema
- Runtime Environment
- execute: Running Tasks as One-Off Builds

Builds
- builds: Showing build history
- intercept: Accessing a running or recent build's steps
- watch: View logs of in-progress builds
- abort-build: Aborting a running build of a job

Pipelines
  * Schema
  * set-pipeline: Configuring Pipelines
  * Parameters & Credentials
- Managing Pipelines
  * pipelines: Listing configured pipelines
  * rename-pipeline: Rename a pipeline
  * pause-pipeline: Preventing new pipeline activity
  * unpause-pipeline: Resuming pipeline activity
  * expose-pipeline: Making a pipeline publicly viewable
  * hide-pipeline: Hiding a pipeline from the public
  * get-pipeline: Fetching a pipeline's configuration
  * destroy-pipeline: Removing Pipelines
  * validate-pipeline: Validate a pipeline config
  * format-pipeline: Canonically format a pipeline config

Jobs
  * Schema
- Managing Jobs
  * jobs: Listing configured jobs
  * trigger-job: Triggering a new build of a job
  * pause-job: Preventing new job activity
  * unpause-job: Resuming job activity

Resources
  * Schema
- Managing Resources
  * check-resource: Trigger discovery of new versions
  * pause-resource: Prevent resource checks
  * unpause-resource: Resume resource checks

Resource Types
  * Schema
- Implementing a Resource Type

Teams
  * teams: Listing configured Teams
  * set-team: Creating and updating Teams
  * destroy-team: Removing Teams
- Auth Types
- Pipeline and Build Visibility
- Security Caveats

Administration
- workers: Listing registered workers
- prune-worker: Reap a non-running worker
- containers: Listing active containers
- volumes: Listing active volumes

Miscellaneous
- checklist: Generate Checkman definition files
