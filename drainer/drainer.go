package drainer

import (
	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
	"github.com/concourse/worker/ssh"
)

type Drainer struct {
	SSHRunner    ssh.Runner
	IsShutdown   bool
	WatchProcess WatchProcess
	WaitInterval time.Duration
	Clock        clock.Clock
	Timeout      *time.Duration
}

func (d *Drainer) Drain(logger lager.Logger) error {
	var tryUntil time.Time
	if d.Timeout != nil {
		tryUntil = d.Clock.Now().Add(*d.Timeout)
	}

	for {
		processIsRunning, err := d.WatchProcess.IsRunning(logger)
		if err != nil {
			logger.Error("failed-to-check-if-process-is-running", err)
			return err
		}

		if !processIsRunning {
			logger.Debug("process-is-not-running-exiting")
			return nil
		}

		if d.Timeout != nil && d.Clock.Now().After(tryUntil) {
			logger.Debug("drain-timeout-passed-exiting")

			if d.IsShutdown {
				err := d.SSHRunner.DeleteWorker(logger)
				if err != nil {
					if err == ssh.ErrFailedToReachAnyTSA {
						logger.Debug(err.Error())
						return nil
					}

					logger.Error("failed-to-delete-worker", err)
					return err
				}
			}

			return nil
		}

		if d.IsShutdown {
			err := d.SSHRunner.RetireWorker(logger)
			if err != nil {
				if err == ssh.ErrFailedToReachAnyTSA {
					logger.Debug(err.Error())
					return nil
				}

				logger.Error("failed-to-retire-worker", err)
			}
		} else {
			err = d.SSHRunner.LandWorker(logger)
			if err != nil {
				if err == ssh.ErrFailedToReachAnyTSA {
					logger.Debug(err.Error())
					return nil
				}

				logger.Error("failed-to-land-worker", err)
			}
		}

		d.Clock.Sleep(d.WaitInterval)
	}
}
