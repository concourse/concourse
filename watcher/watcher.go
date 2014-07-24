package watcher

import (
	"os"

	"github.com/concourse/atc/config"

	"github.com/concourse/atc/watchman"
)

type Watcher struct {
	resources config.Resources
	watchman  watchman.Watchman
}

func NewWatcher(resources config.Resources, watchman watchman.Watchman) *Watcher {
	return &Watcher{
		resources: resources,
		watchman:  watchman,
	}
}

func (watcher Watcher) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	for _, resource := range watcher.resources {
		watcher.watchman.Watch(resource)
	}

	close(ready)

	<-signals

	watcher.watchman.Stop()

	return nil
}
