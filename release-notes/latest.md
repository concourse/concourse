#### <sub><sup><a name="v560-note-4480" href="#v560-note-4480">:link:</a></sup></sub> feature

* @ProvoK added support for a `?title=` query parameter on the pipeline/job
  badge endpoints! Now you can make it say something other than "build". #4480

  ![badge](https://ci.concourse-ci.org/api/v1/teams/main/pipelines/concourse/badge?title=check%20it%20out)

#### <sub><sup><a name="v561-note-4518" href="#v561-note-4518">:link:</a></sup></sub> fix

* @evanchaoli added a feature to stop ATC from attempting to renew Vault leases that are not renewable #4518.

#### <sub><sup><a name="v561-note-4516" href="#v561-note-4516">:link:</a></sup></sub> fix

* Add 5 minute timeout for baggageclaim destroy calls.

#### <sub><sup><a name="v561-note-4334" href="#v561-note-4334">:link:</a></sup></sub> fix

* @aledeganopix4d added a feature sort pipelines alphabetically.
