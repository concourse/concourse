package worker

import (
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/worker/beacon"
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

func BeaconRunner(logger lager.Logger, beacon beacon.BeaconClient) ifrit.Runner {
	return restart.Restarter{
		Runner: ifrit.RunFunc(beacon.Register),
		Load: func(prevRunner ifrit.Runner, prevErr error) ifrit.Runner {
			if prevErr == nil {
				return nil
			}

			if _, ok := prevErr.(*ssh.ExitError); !ok {
				logger.Error("restarting", prevErr)
				time.Sleep(5 * time.Second)
				return ifrit.RunFunc(beacon.Register)
			}
			return nil
		},
	}
}
