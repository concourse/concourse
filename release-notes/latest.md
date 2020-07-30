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
