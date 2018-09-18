package sweeper

import (
	"os"
	"time"

	"code.cloudfoundry.org/garden"
	gdnClient "code.cloudfoundry.org/garden/client"

	"code.cloudfoundry.org/garden/client/connection"
	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/worker/beacon"
	"github.com/tedsuo/ifrit"
)

// Command is the struct that holds the properties for the mark and sweep command
type Command struct {
	Logger       lager.Logger
	BeaconClient beacon.BeaconClient
	GCInterval   time.Duration
	GardenClient garden.Client
}

// NewSweeperRunner provides the ifrit runner that marks and sweeps the containers
func NewSweeperRunner(logger lager.Logger,
	atcWorker atc.Worker,
	config beacon.Config,
) ifrit.Runner {
	logger.Info("sweep-starting")

	client := beacon.NewSSHClient(logger.Session("beacon-client"), config)

	beaconC := &beacon.Beacon{
		Logger:    logger,
		Worker:    atcWorker,
		Client:    client,
		KeepAlive: false, // disable keepalive for mark and sweep calls
	}

	scmd := &Command{
		BeaconClient: beaconC,
		Logger:       logger.Session("sweeper"),
		GCInterval:   30 * time.Second,
		GardenClient: gdnClient.New(connection.New("tcp", atcWorker.GardenAddr)),
	}
	return scmd
}

// Run invokes the process of marking and sweeping containers
// First worker will call atc to collect list of containers to be removed
// and then worker will report its state of current containers for
// atc to remove containers in DB. This cycle is triggered every GCInterval sec
func (cmd *Command) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	timer := time.NewTicker(cmd.GCInterval)
	close(ready)

	for {
		select {
		case <-timer.C:

			err := cmd.BeaconClient.ReportContainers(cmd.GardenClient)
			if err != nil {
				cmd.Logger.Error("failed-to-report-containers", err)
			}

			err = cmd.BeaconClient.ReportVolumes()
			if err != nil {
				cmd.Logger.Error("failed-to-report-volumes", err)
			}

			err = cmd.BeaconClient.SweepContainers(cmd.GardenClient)
			if err != nil {
				cmd.Logger.Error("failed-to-sweep-containers", err)
			}

			err = cmd.BeaconClient.SweepVolumes()
			if err != nil {
				cmd.Logger.Error("failed-to-sweep-volumes", err)
			}

		case <-signals:
			cmd.Logger.Info("exiting-from-mark-and-sweep")
			return nil
		}
	}
}
