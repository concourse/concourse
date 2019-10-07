# worker

*worker - container orchestrator/ worker artifact management/ build executor*

![](https://upload.wikimedia.org/wikipedia/commons/thumb/b/b8/Otto_Lilienthal_gliding_experiment_ppmsca.02546.jpg/640px-Otto_Lilienthal_gliding_experiment_ppmsca.02546.jpg)

[from](https://commons.wikimedia.org/wiki/File:Otto_Lilienthal_gliding_experiment_ppmsca.02546.jpg) the [United States Library of Congress's](https://www.loc.gov/) [Prints and Photographs division](https://www.loc.gov/rr/print/)






  ## about

  *worker* is the workhorse of Concourse. It's responsible for the creation
  and deletion of the containers in which the pipeline operations
  (get, check, put, task) are executed.

  A worker node registers with the web node(s) and is then used for executing
  builds and performing resource checks.

  The ATC component in the web node(s) decides how to allocate containers
  to workers that have been registered in the pool, using the configured
  container-placement-strategy. It also manages the container deletion on the workers
  via database calls and the Garden API on the workers.

  A worker node runs the following 2 GO API's

  * [Garden](https://github.com/cloudfoundry-incubator/garden) is a generic
    interface for orchestrating containers remotely on a worker

  * [Baggageclaim](https://github.com/concourse/baggageclaim) is a server for
    managing caches and artifacts on the workers


  It can be scaled horizontally in order to scale the system.

  [More Info](https://concourse-ci.org/concourse-worker.html)
