package watcher

import (
	"log"
	"os"
	"time"

	"github.com/tedsuo/router"
	"github.com/winston-ci/winston/config"
	"github.com/winston-ci/winston/db"
	"github.com/winston-ci/winston/resources"
	"github.com/winston-ci/winston/watchman"
)

type Watcher struct {
	jobs      config.Jobs
	resources config.Resources
	db        db.DB
	prole     *router.RequestGenerator
	watchman  watchman.Watchman
}

func NewWatcher(
	jobs config.Jobs,
	resources config.Resources,
	db db.DB,
	prole *router.RequestGenerator,
	watchman watchman.Watchman,
) *Watcher {
	return &Watcher{
		jobs:      jobs,
		resources: resources,
		db:        db,
		prole:     prole,
		watchman:  watchman,
	}
}

func (watcher Watcher) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	for _, job := range watcher.jobs {
		for _, input := range job.Inputs {
			resource, found := watcher.resources.Lookup(input.Resource)
			if !found {
				log.Fatalln("unknown resource:", input.Resource)
			}

			current, err := watcher.db.GetCurrentVersion(job.Name, input.Resource)
			if err != nil {
				current = nil
			}

			var checker resources.Checker
			if len(input.Passed) == 0 {
				checker = resources.NewProleChecker(watcher.prole)
			} else {
				checker = resources.NewWinstonChecker(watcher.db, input.Passed)
			}

			watcher.watchman.Watch(job, resource, current, checker, input.LatestOnly, time.Minute)
		}
	}

	close(ready)

	<-signals

	watcher.watchman.Stop()

	return nil
}
