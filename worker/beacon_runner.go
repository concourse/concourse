package worker

import (
	"os"
	"os/signal"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/worker/beacon"
	"github.com/concourse/concourse/worker/drain"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/restart"
	"golang.org/x/crypto/ssh"
)

func NewBeacon(logger lager.Logger, worker atc.Worker, config beacon.Config) beacon.BeaconClient {
	logger = logger.Session("beacon")
	logger.Debug("setting-up-beacon-runner")
	client := beacon.NewSSHClient(logger.Session("beacon-client"), config)

	return &beacon.Beacon{
		Logger:           logger,
		Worker:           worker,
		Client:           client,
		GardenAddr:       config.GardenForwardAddr,
		BaggageclaimAddr: config.BaggageclaimForwardAddr,
		RegistrationMode: config.Registration.Mode,
		KeepAlive:        true,
		RebalanceTime:    config.Registration.RebalanceTime,
	}
}

func BeaconRunner(logger lager.Logger, beaconClient beacon.BeaconClient) ifrit.Runner {
	signals := make(chan os.Signal, 2)
	signal.Notify(signals, drain.Signals...)

	runner := &drain.Runner{
		Logger:       logger.Session("drain"),
		Beacon:       beaconClient,
		Runner:       ifrit.RunFunc(beaconClient.Register),
		DrainSignals: signals,
	}

	return restart.Restarter{
		Runner: runner,
		Load: func(prevRunner ifrit.Runner, prevErr error) ifrit.Runner {
			if prevErr == nil {
				return nil
			}

			if prevErr == beacon.ErrAllGatewaysUnreachable && prevRunner.(*drain.Runner).Drained() {
				// this could happen if the whole deployment is being deleted. in this
				// case, we should just exit and stop retrying, because draining can't
				// complete anyway.
				logger.Info("exiting", lager.Data{
					"reason": "all SSH gateways disappeared while draining",
				})
				return nil
			}

			if _, ok := prevErr.(*ssh.ExitError); ok {
				// the gateway caused the process to exit, either because the worker
				// has landed, retired, or is ephemeral and stalled (resulting in its
				// deletion)
				logger.Info("exiting", lager.Data{
					"reason": "registration process exited via SSH gateway",
				})
				return nil
			}

			logger.Error("failed", prevErr)

			time.Sleep(5 * time.Second)

			logger.Info("restarting")

			return runner
		},
	}
}
