package worker

import (
	"os"
	"os/signal"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/tsa"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/restart"
	"golang.org/x/crypto/ssh"
)

func NewBeaconRunner(
	logger lager.Logger,
	tsaClient *tsa.Client,
	rebalanceInterval time.Duration,
	connectionDrainTimeout time.Duration,
	gardenAddr string,
	baggageclaimAddr string,
) ifrit.Runner {
	signals := make(chan os.Signal, 2)
	signal.Notify(signals, drainSignals...)

	beacon := &Beacon{
		Logger: logger.Session("beacon"),

		Client: tsaClient,

		RebalanceInterval:      rebalanceInterval,
		ConnectionDrainTimeout: connectionDrainTimeout,

		DrainSignals: signals,

		LocalGardenNetwork: "tcp",
		LocalGardenAddr:    gardenAddr,

		LocalBaggageclaimNetwork: "tcp",
		LocalBaggageclaimAddr:    baggageclaimAddr,
	}

	return restart.Restarter{
		Runner: beacon,
		Load: func(prevRunner ifrit.Runner, prevErr error) ifrit.Runner {
			if prevErr == nil {
				return nil
			}

			if prevErr == tsa.ErrAllGatewaysUnreachable && beacon.Drained() {
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

			return beacon
		},
	}
}
