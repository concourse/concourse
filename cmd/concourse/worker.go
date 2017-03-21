package main

import (
	"fmt"
	"os"
	"time"

	"golang.org/x/crypto/ssh"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/concourse/baggageclaim/baggageclaimcmd"
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

	CertificatesPath string `long:"certificates-path" default:"/etc/ssl/certs" description:"Path to directory containing certificates that will be shared in containers at /etc/ssl/certs."`

	WorkDir string `long:"work-dir" required:"true" description:"Directory in which to place container data."`

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

	worker, gardenRunner, err := cmd.gardenRunner(logger.Session("garden"), args)
	if err != nil {
		return err
	}

	baggageclaimRunner, err := cmd.baggageclaimRunner(logger.Session("baggageclaim"))
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

func (cmd *WorkerCommand) workerName() (string, error) {
	if cmd.Name != "" {
		return cmd.Name, nil
	}

	return os.Hostname()
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
