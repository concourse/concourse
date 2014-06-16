package timedtrigger

import (
	"os"
	"sync"
	"time"

	"github.com/tedsuo/ifrit"

	"github.com/winston-ci/winston/config"
	"github.com/winston-ci/winston/queue"
)

func NewTimer(jobs config.Jobs, queuer queue.Queuer) ifrit.Runner {
	return ifrit.RunFunc(func(signals <-chan os.Signal, ready chan<- struct{}) error {
		ticking := &sync.WaitGroup{}

		stop := make(chan struct{})

		for _, job := range jobs {
			if job.TriggerEvery == 0 {
				continue
			}

			timer := time.NewTicker(time.Duration(job.TriggerEvery))
			ticking.Add(1)

			go func(job config.Job) {
				defer ticking.Done()

				for {
					select {
					case <-timer.C:
						queuer.Trigger(job)
					case <-stop:
						return
					}
				}
			}(job)
		}

		close(ready)

		<-signals

		close(stop)

		ticking.Wait()

		return nil
	})
}
