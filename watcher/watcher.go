package watcher

import (
	"os"
	"time"

	"github.com/concourse/atc/config"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/resources"
	"github.com/concourse/atc/watchman"
	"github.com/tedsuo/rata"
)

type Watcher struct {
	jobs          config.Jobs
	resources     config.Resources
	db            db.DB
	turbine       *rata.RequestGenerator
	watchman      watchman.Watchman
	checkInterval time.Duration
}

func NewWatcher(
	jobs config.Jobs,
	resources config.Resources,
	db db.DB,
	turbine *rata.RequestGenerator,
	watchman watchman.Watchman,
	checkInterval time.Duration,
) *Watcher {
	return &Watcher{
		jobs:          jobs,
		resources:     resources,
		db:            db,
		turbine:       turbine,
		watchman:      watchman,
		checkInterval: checkInterval,
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
				panic("unknown resource: " + input.Resource)
			}

			var checker resources.Checker
			if len(input.Passed) == 0 {
				checker = resources.NewTurbineChecker(watcher.turbine)
			} else {
				checker = resources.NewWinstonChecker(watcher.db, input.Passed)
			}

			watcher.watchman.Watch(job, resource, checker, input.EachVersion, watcher.checkInterval)
		}
	}

	close(ready)

	<-signals

	watcher.watchman.Stop()

	return nil
}
