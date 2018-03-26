package main

import (
	"fmt"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/baggageclaim/baggageclaimcmd"
	"github.com/concourse/bin/bindata"
	"github.com/concourse/flag"
	concourseWorker "github.com/concourse/worker"
	workerConfig "github.com/concourse/worker/start"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/grouper"
	"github.com/tedsuo/ifrit/sigmon"
)

type WorkerCommand struct {
	Worker workerConfig.Config
	Certs  Certs

	WorkDir flag.Dir `long:"work-dir" required:"true" description:"Directory in which to place container data."`

	BindIP   flag.IP `long:"bind-ip"   default:"127.0.0.1" description:"IP address on which to listen for the Garden server."`
	BindPort uint16  `long:"bind-port" default:"7777"      description:"Port on which to listen for the Garden server."`
	PeerIP   flag.IP `long:"peer-ip" description:"IP used to reach this worker from the ATC nodes."`

	Garden GardenBackend `group:"Garden Configuration" namespace:"garden"`

	Baggageclaim baggageclaimcmd.BaggageclaimCommand `group:"Baggageclaim Configuration" namespace:"baggageclaim"`

	Metrics struct {
		YellerAPIKey      string `long:"yeller-api-key"     description:"Yeller API key. If specified, all errors logged will be emitted."`
		YellerEnvironment string `long:"yeller-environment" description:"Environment to tag on all Yeller events emitted."`
	} `group:"Metrics & Diagnostics"`
}

func (cmd *WorkerCommand) Execute(args []string) error {
	runner, err := cmd.Runner(args)
	if err != nil {
		return err
	}

	return <-ifrit.Invoke(sigmon.New(runner)).Wait()
}

func (cmd *WorkerCommand) Runner(args []string) (ifrit.Runner, error) {
	logger := lager.NewLogger("worker")
	logger.RegisterSink(lager.NewWriterSink(os.Stdout, lager.INFO))

	hasAssets, err := cmd.setup(logger.Session("setup"))
	if err != nil {
		return nil, err
	}

	worker, gardenRunner, err := cmd.gardenRunner(logger.Session("garden"), hasAssets)
	if err != nil {
		return nil, err
	}

	worker.Version = WorkerVersion

	baggageclaimRunner, err := cmd.baggageclaimRunner(logger.Session("baggageclaim"), hasAssets)
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

	if cmd.Worker.TSA.TSAConfig.WorkerPrivateKey.PrivateKey != nil {
		if cmd.PeerIP.IP != nil {
			worker.GardenAddr = fmt.Sprintf("%s:%d", cmd.PeerIP.IP, cmd.BindPort)
			worker.BaggageclaimURL = fmt.Sprintf("http://%s:%d", cmd.PeerIP.IP, cmd.Baggageclaim.BindPort)
		} else {
			worker.GardenAddr = fmt.Sprintf("%s:%d", cmd.BindIP.IP, cmd.BindPort)
			worker.BaggageclaimURL = fmt.Sprintf("http://%s:%d", cmd.Baggageclaim.BindIP.IP, cmd.Baggageclaim.BindPort)
		}

		members = append(members, grouper.Member{
			Name: "beacon",
			Runner: concourseWorker.BeaconRunner(
				logger.Session("beacon"),
				worker,
				cmd.Worker.TSA,
			),
		})
	}

	return grouper.NewParallel(os.Interrupt, members), nil
}

func (cmd *WorkerCommand) assetPath(paths ...string) string {
	return filepath.Join(append([]string{cmd.WorkDir.Path(), Version, "assets"}, paths...)...)
}

func (cmd *WorkerCommand) setup(logger lager.Logger) (bool, error) {
	okMarker := cmd.assetPath("ok")

	_, err := os.Stat(okMarker)
	if err == nil {
		logger.Info("already-done")
		return true, nil
	}

	_, err = bindata.AssetDir("assets")
	if err != nil {
		logger.Info("no-assets")
		return false, nil
	}

	logger.Info("unpacking")

	err = bindata.RestoreAssets(filepath.Split(cmd.assetPath()))
	if err != nil {
		logger.Error("failed-to-unpack", err)
		return false, err
	}

	_, err = os.Stat(cmd.assetPath())
	if os.IsNotExist(err) {
		logger.Info("no-assets")
		return false, nil
	}

	ok, err := os.Create(okMarker)
	if err != nil {
		logger.Error("failed-to-create-ok-marker", err)
		return false, err
	}

	err = ok.Close()
	if err != nil {
		logger.Error("failed-to-close-ok-marker", err)
		return false, err
	}

	logger.Info("done")

	return true, nil
}

func (cmd *WorkerCommand) workerName() (string, error) {
	if cmd.Worker.Name != "" {
		return cmd.Worker.Name, nil
	}

	return os.Hostname()
}

func (cmd *WorkerCommand) baggageclaimRunner(logger lager.Logger, hasAssets bool) (ifrit.Runner, error) {
	volumesDir := filepath.Join(cmd.WorkDir.Path(), "volumes")

	err := os.MkdirAll(volumesDir, 0755)
	if err != nil {
		return nil, err
	}

	cmd.Baggageclaim.Metrics = cmd.Metrics
	cmd.Baggageclaim.VolumesDir = flag.Dir(volumesDir)

	cmd.Baggageclaim.OverlaysDir = filepath.Join(cmd.WorkDir.Path(), "overlays")

	if hasAssets {
		cmd.Baggageclaim.MkfsBin = cmd.assetPath("btrfs", "mkfs.btrfs")
		cmd.Baggageclaim.BtrfsBin = cmd.assetPath("btrfs", "btrfs")
	}

	return cmd.Baggageclaim.Runner(nil)
}
