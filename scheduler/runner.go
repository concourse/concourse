package scheduler

import (
	"os"
	"time"

	"github.com/concourse/atc/config"
)

type Runner struct {
	Noop bool

	Scheduler *Scheduler
	Jobs      config.Jobs
}

func (runner *Runner) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	close(ready)

	if runner.Noop {
		<-signals
		return nil
	}

	for {
		select {
		case <-time.After(10 * time.Second):
			for _, job := range runner.Jobs {
				runner.Scheduler.TryNextPendingBuild(job)
				runner.Scheduler.BuildLatestInputs(job)
			}

		case <-signals:
			return nil
		}
	}

	return nil
}
