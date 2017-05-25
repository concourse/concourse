package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/crypto/ssh"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/concourse/baggageclaim/baggageclaimcmd"
	"github.com/concourse/bin/bindata"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/grouper"
	"github.com/tedsuo/ifrit/restart"
	"github.com/tedsuo/ifrit/sigmon"
)

type WorkerCommand struct {
	Name string   `long:"name" description:"The name to set for the worker during registration. If not specified, the hostname will be used."`
	Tags []string `long:"tag" description:"A tag to set during registration. Can be specified multiple times."`

	TeamName string `long:"team" description:"The name of the team that this worker will be assigned to."`

	HTTPProxy  URLFlag  `long:"http-proxy"  env:"http_proxy"                  description:"HTTP proxy endpoint to use for containers."`
	HTTPSProxy URLFlag  `long:"https-proxy" env:"https_proxy"                 description:"HTTPS proxy endpoint to use for containers."`
	NoProxy    []string `long:"no-proxy"    env:"no_proxy"    env-delim:","   description:"Blacklist of addresses to skip the proxy when reaching."`

	WorkDir DirFlag `long:"work-dir" required:"true" description:"Directory in which to place container data."`

	BindIP   IPFlag `long:"bind-ip"   default:"127.0.0.1" description:"IP address on which to listen for the Garden server."`
	BindPort uint16 `long:"bind-port" default:"7777"      description:"Port on which to listen for the Garden server."`

	PeerIP IPFlag `long:"peer-ip" description:"IP used to reach this worker from the ATC nodes. If omitted, the worker will be forwarded through the SSH connection to the TSA."`

	Garden GardenBackend `group:"Garden Configuration" namespace:"garden"`

	Baggageclaim baggageclaimcmd.BaggageclaimCommand `group:"Baggageclaim Configuration" namespace:"baggageclaim"`

	TSA BeaconConfig `group:"TSA Configuration" namespace:"tsa"`

	Metrics struct {
		YellerAPIKey      string `long:"yeller-api-key"     description:"Yeller API key. If specified, all errors logged will be emitted."`
		YellerEnvironment string `long:"yeller-environment" description:"Environment to tag on all Yeller events emitted."`
	} `group:"Metrics & Diagnostics"`
}

func (cmd *WorkerCommand) Execute(args []string) error {
	logger := lager.NewLogger("worker")
	logger.RegisterSink(lager.NewWriterSink(os.Stdout, lager.INFO))

	hasAssets, err := cmd.setup(logger.Session("setup"))
	if err != nil {
		return err
	}

	worker, gardenRunner, err := cmd.gardenRunner(logger.Session("garden"), hasAssets)
	if err != nil {
		return err
	}

	worker.Version = WorkerVersion

	baggageclaimRunner, err := cmd.baggageclaimRunner(logger.Session("baggageclaim"), hasAssets)
	if err != nil {
		return err
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

	if cmd.TSA.WorkerPrivateKey != "" {
		members = append(members, grouper.Member{
			Name:   "beacon",
			Runner: cmd.beaconRunner(logger.Session("beacon"), worker),
		})
	}

	runner := sigmon.New(grouper.NewParallel(os.Interrupt, members))

	return <-ifrit.Invoke(runner).Wait()
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
	if cmd.Name != "" {
		return cmd.Name, nil
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
	cmd.Baggageclaim.VolumesDir = baggageclaimcmd.DirFlag(volumesDir)

	cmd.Baggageclaim.OverlaysDir = filepath.Join(cmd.WorkDir.Path(), "overlays")

	if hasAssets {
		cmd.Baggageclaim.MkfsBin = cmd.assetPath("btrfs", "mkfs.btrfs")
		cmd.Baggageclaim.BtrfsBin = cmd.assetPath("btrfs", "btrfs")
	}

	return cmd.Baggageclaim.Runner(nil)
}

func (cmd *WorkerCommand) beaconRunner(logger lager.Logger, worker atc.Worker) ifrit.Runner {
	beacon := Beacon{
		Logger: logger,
		Config: cmd.TSA,
	}

	var beaconRunner ifrit.RunFunc
	if cmd.PeerIP != nil {
		worker.GardenAddr = fmt.Sprintf("%s:%d", cmd.PeerIP.IP(), cmd.BindPort)
		worker.BaggageclaimURL = fmt.Sprintf("http://%s:%d", cmd.PeerIP.IP(), cmd.Baggageclaim.BindPort)
		beaconRunner = beacon.Register
	} else {
		worker.GardenAddr = fmt.Sprintf("%s:%d", cmd.BindIP.IP(), cmd.BindPort)
		worker.BaggageclaimURL = fmt.Sprintf("http://%s:%d", cmd.Baggageclaim.BindIP.IP(), cmd.Baggageclaim.BindPort)
		beaconRunner = beacon.Forward
	}

	beacon.Worker = worker

	return restart.Restarter{
		Runner: beaconRunner,
		Load: func(prevRunner ifrit.Runner, prevErr error) ifrit.Runner {
			if prevErr == nil {
				return nil
			}

			if _, ok := prevErr.(*ssh.ExitError); !ok {
				logger.Error("restarting", prevErr)
				time.Sleep(5 * time.Second)
				return beaconRunner
			}

			return nil
		},
	}
}
