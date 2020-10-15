#### <sub><sup><a name="5830" href="#5830">:link:</a></sup></sub> fix

* Fix a validation issue where a step can be set with 0 attempts causing the ATC to panic. #5830

#### <sub><sup><a name="5842" href="#5842">:link:</a></sup></sub> feature

* Added recover for panic error that used to crash the cluster. Now it should be less easy to panic (we hope!) and if it does, panic error could be found on Stderr and log. #5842

#### <sub><sup><a name="5810" href="#5810">:link:</a></sup></sub> feature

* Reduce the allowed character set for Concourse valid identifiers. Only prints warnings instead of errors as a first step. #5810
* Add `--team` flag for `fly pause-pipeline` command. #5917
* Add `--team` flag for `fly hide-pipeline` command. #5917

#### <sub><sup><a name="5854" href="#5854">:link:</a></sup></sub> feature

* Automatically archive pipelines set by a set_pipeline step that meets one of the following criteria: #5854
  * set_pipeline step is removed from job
  * Job that set pipeline is deleted
  * Parent pipeline is deleted

#### <sub><sup><a name="5846" href="#5810">:link:</a></sup></sub> feature

* @evanchaoli Enhanced build log page as well as `fly watch` to display worker name for `get/put/task` steps. #5846

#### <sub><sup><a name="5146" href="#5146">:link:</a></sup></sub> feature

* Refactor TSA to use Concourse's gclient which has a configurable timeout Issue: #5146 PR: #5845

#### <sub><sup><a name="5981" href="#5981">:link:</a></sup></sub> feature

* Enhance `task_waiting` metric to export labels in Prometheus for: platform, worker tags and team of the tasks awaiting execution.

  A new metric called `tasks_wait_duration_bucket` is also added to express as quantiles the average time spent by tasks awaiting execution. PR: #5981
  ![Example graph for the task wait time histograms.](https://user-images.githubusercontent.com/40891147/89990749-189d2600-dc83-11ea-8fde-ae579fdb0a0a.png)
