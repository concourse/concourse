package main

import (
	"fmt"
	"os"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/concourse/worker/beacon"
	flags "github.com/jessevdk/go-flags"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/restart"
	"golang.org/x/crypto/ssh"
)

// overridden via linker flags
var Version = "0.0.0-dev"

type WorkerCommand struct {
	GardenAddr      string `long:"garden-addr" required:"true" `
	BaggageclaimURL string `long:"baggageclaim-url" required:"true" `

	Platform string   `long:"platform"`
	Tags     []string `long:"tags"`
	Team     string   `long:"team"`
	Name     string   `long:"name"`
	Version  string   `long:"version"`

	BeaconConfig beacon.Config `group:"Beacon Configuration" namespace:"beacon"`
}

func main() {
	var cmd WorkerCommand

	parser := flags.NewParser(&cmd, flags.HelpFlag|flags.PassDoubleDash)
	parser.NamespaceDelimiter = "-"

	_, err := parser.Parse()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	runner := BeaconRunner(lager.NewLogger("beacon"), atc.Worker{
		GardenAddr:      cmd.GardenAddr,
		BaggageclaimURL: cmd.BaggageclaimURL,
		Platform:        cmd.Platform,
		Tags:            cmd.Tags,
		Team:            cmd.Team,
		Name:            cmd.Name,
		Version:         cmd.Version,
	})

	ifrit.Invoke(runner).Wait()
}

func BeaconRunner(logger lager.Logger, worker atc.Worker) ifrit.Runner {

	var client beacon.Client

	beacon := beacon.Beacon{
		Logger: logger,
		Worker: worker,
		Client: client,
	}

	var beaconRunner ifrit.RunFunc = beacon.Register

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
