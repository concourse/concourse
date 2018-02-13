package main

import (
	"fmt"
	"os"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/concourse/worker"
	"github.com/concourse/worker/beacon"
	flags "github.com/jessevdk/go-flags"
	"github.com/tedsuo/ifrit"
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

	logger := lager.NewLogger("worker")
	//TODO support changing the log level
	logger.RegisterSink(lager.NewWriterSink(os.Stdout, lager.INFO))
	runner := worker.BeaconRunner(logger.Session("beacon"), atc.Worker{
		GardenAddr:      cmd.GardenAddr,
		BaggageclaimURL: cmd.BaggageclaimURL,
		Platform:        cmd.Platform,
		Tags:            cmd.Tags,
		Team:            cmd.Team,
		Name:            cmd.Name,
		Version:         cmd.Version,
	}, cmd.BeaconConfig)

	<-ifrit.Invoke(runner).Wait()
}
