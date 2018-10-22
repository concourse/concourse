package main

import (
	"fmt"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/baggageclaim/baggageclaimcmd"
	"github.com/concourse/concourse"
	concourseWorker "github.com/concourse/concourse/worker"
	"github.com/concourse/concourse/worker/beacon"
	workerConfig "github.com/concourse/concourse/worker/start"
	"github.com/concourse/concourse/worker/sweeper"
	"github.com/concourse/concourse/worker/tsa"
	"github.com/concourse/flag"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/grouper"
	"github.com/tedsuo/ifrit/sigmon"
)

type WorkerCommand struct {
	Worker workerConfig.Config

	TSA tsa.Config `group:"TSA Configuration" namespace:"tsa"`

	Certs Certs

	WorkDir flag.Dir `long:"work-dir" required:"true" description:"Directory in which to place container data."`

	BindIP   flag.IP `long:"bind-ip"   default:"127.0.0.1" description:"IP address on which to listen for the Garden server."`
	BindPort uint16  `long:"bind-port" default:"7777"      description:"Port on which to listen for the Garden server."`
	PeerIP   flag.IP `long:"peer-ip" description:"IP used to reach this worker from the ATC nodes."`

	Garden GardenBackend `group:"Garden Configuration" namespace:"garden"`

	Baggageclaim baggageclaimcmd.BaggageclaimCommand `group:"Baggageclaim Configuration" namespace:"baggageclaim"`

	ResourceTypes flag.Dir `long:"resource-types" description:"Path to directory containing resource types the worker should advertise."`

	Logger flag.Lager
}

func (cmd *WorkerCommand) Execute(args []string) error {
	runner, err := cmd.Runner(args)
	if err != nil {
		return err
	}

	return <-ifrit.Invoke(sigmon.New(runner)).Wait()
}

func (cmd *WorkerCommand) Runner(args []string) (ifrit.Runner, error) {
	if cmd.ResourceTypes == "" {
		cmd.ResourceTypes = flag.Dir(discoverAsset("resource-types"))
	}

	logger, _ := cmd.Logger.Logger("worker")

	worker, gardenRunner, err := cmd.gardenRunner(logger.Session("garden"))
	if err != nil {
		return nil, err
	}

	worker.Version = concourse.WorkerVersion

	baggageclaimRunner, err := cmd.baggageclaimRunner(logger.Session("baggageclaim"))
	if err != nil {
		return nil, err
	}

	members := grouper.Members{
		{
			Name:   "garden",
			Runner: gardenRunner,
		},
		{
			Name:   "baggageclaim",
			Runner: baggageclaimRunner,
		},
	}

	if cmd.TSA.WorkerPrivateKey != nil {
		beaconConfig := beacon.Config{
			TSAConfig: cmd.TSA,
		}

		if cmd.PeerIP.IP != nil {
			worker.GardenAddr = fmt.Sprintf("%s:%d", cmd.PeerIP.IP, cmd.BindPort)
			worker.BaggageclaimURL = fmt.Sprintf("http://%s:%d", cmd.PeerIP.IP, cmd.Baggageclaim.BindPort)

			beaconConfig.Registration.Mode = "direct"
		} else {
			beaconConfig.Registration.Mode = "forward"
			beaconConfig.GardenForwardAddr = fmt.Sprintf("%s:%d", cmd.BindIP.IP, cmd.BindPort)
			beaconConfig.BaggageclaimForwardAddr = fmt.Sprintf("%s:%d", cmd.Baggageclaim.BindIP.IP, cmd.Baggageclaim.BindPort)

			worker.GardenAddr = beaconConfig.GardenForwardAddr
			worker.BaggageclaimURL = fmt.Sprintf("http://%s", beaconConfig.BaggageclaimForwardAddr)
		}

		members = append(members, grouper.Member{
			Name: "beacon",
			Runner: concourseWorker.BeaconRunner(
				logger.Session("beacon"),
				worker,
				beaconConfig,
			),
		})

		members = append(members, grouper.Member{
			Name: "sweeper",
			Runner: sweeper.NewSweeperRunner(
				logger,
				worker,
				beaconConfig,
			),
		})
	}

	return grouper.NewParallel(os.Interrupt, members), nil
}

func (cmd *WorkerCommand) workerName() (string, error) {
	if cmd.Worker.Name != "" {
		return cmd.Worker.Name, nil
	}

	return os.Hostname()
}

func (cmd *WorkerCommand) baggageclaimRunner(logger lager.Logger) (ifrit.Runner, error) {
	volumesDir := filepath.Join(cmd.WorkDir.Path(), "volumes")

	err := os.MkdirAll(volumesDir, 0755)
	if err != nil {
		return nil, err
	}

	cmd.Baggageclaim.VolumesDir = flag.Dir(volumesDir)

	cmd.Baggageclaim.OverlaysDir = filepath.Join(cmd.WorkDir.Path(), "overlays")

	return cmd.Baggageclaim.Runner(nil)
}
