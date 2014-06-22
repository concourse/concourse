package watcher

import (
	"log"
	"os"
	"time"

	"github.com/concourse/atc/config"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/resources"
	"github.com/concourse/atc/watchman"
	"github.com/tedsuo/router"
)

type Watcher struct {
	jobs      config.Jobs
	resources config.Resources
	db        db.DB
	turbine   *router.RequestGenerator
	watchman  watchman.Watchman
}

func NewWatcher(
	jobs config.Jobs,
	resources config.Resources,
	db db.DB,
	turbine *router.RequestGenerator,
	watchman watchman.Watchman,
) *Watcher {
	return &Watcher{
		jobs:      jobs,
		resources: resources,
		db:        db,
		turbine:   turbine,
		watchman:  watchman,
	}
}

func (watcher Watcher) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	for _, job := range watcher.jobs {
		for _, input := range job.Inputs {
			if input.DontCheck {
				continue
			}

			resource, found := watcher.resources.Lookup(input.Resource)
			if !found {
				log.Fatalln("unknown resource:", input.Resource)
			}

			var checker resources.Checker
			if len(input.Passed) == 0 {
				checker = resources.NewTurbineChecker(watcher.turbine)
			} else {
				checker = resources.NewWinstonChecker(watcher.db, input.Passed)
			}

			watcher.watchman.Watch(job, resource, checker, input.EachVersion, time.Minute)
		}
	}

	close(ready)

	<-signals

	watcher.watchman.Stop()

	return nil
}
