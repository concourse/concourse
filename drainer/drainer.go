package drainer

import (
	"os"
	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
	"github.com/concourse/worker/beacon"
)

type Drainer struct {
	BeaconClient beacon.BeaconClient
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

				signals := make(chan os.Signal)
				readyChan := make(chan struct{})

				err := d.BeaconClient.DeleteWorker(signals, readyChan)
				if err != nil {
					if err == beacon.ErrFailedToReachAnyTSA {
						logger.Debug(err.Error())
						return nil
					}

					logger.Error("failed-to-delete-worker", err)
					return err
				}
				logger.Info("finished-deleting-worker")
			}

			return nil
		}

		if d.IsShutdown {
			signals := make(chan os.Signal)
			readyChan := make(chan struct{})
			err := d.BeaconClient.RetireWorker(signals, readyChan)
			if err != nil {
				if err == beacon.ErrFailedToReachAnyTSA {
					logger.Debug(err.Error())
					return nil
				}

				logger.Error("failed-to-retire-worker", err)
			}
			logger.Info("finished-retiring-worker")
		} else {
			signals := make(chan os.Signal)
			readyChan := make(chan struct{})
			err = d.BeaconClient.LandWorker(signals, readyChan)
			if err != nil {
				if err == beacon.ErrFailedToReachAnyTSA {
					logger.Debug(err.Error())
					return nil
				}
				logger.Error("failed-to-land-worker", err)
			}

			logger.Info("finished-landing-worker")
		}
		d.Clock.Sleep(d.WaitInterval)
	}
}
