package main

import (
	"os"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/concourse/flag"
	"github.com/concourse/worker"
	"github.com/concourse/worker/beacon"
	"github.com/tedsuo/ifrit"
)

type BeaconConfigRequired struct {
	Host             []string            `long:"host" required:"true" default:"127.0.0.1:2222" description:"TSA host to negotiate the worker draining through."`
	PublicKey        flag.AuthorizedKeys `long:"public-key" required:"true" description:"File containing a public key to expect from the TSA."`
	WorkerPrivateKey flag.PrivateKey     `long:"worker-private-key" required:"true" description:"File containing the private key to use when authenticating to the TSA."`
}

func (b BeaconConfigRequired) canonical() beacon.Config {
	return beacon.Config{
		Host:             b.Host,
		PublicKey:        b.PublicKey,
		WorkerPrivateKey: b.WorkerPrivateKey,
	}
}

type LandWorkerCommand struct {
	TSA BeaconConfigRequired `group:"TSA Configuration" namespace:"tsa" required:"true"`

	WorkerName string `long:"name" required:"true" description:"The name of the worker you wish to land."`
}

func (cmd *LandWorkerCommand) Execute(args []string) error {
	logger := lager.NewLogger("land-worker")
	logger.RegisterSink(lager.NewWriterSink(os.Stdout, lager.DEBUG))

	landWorkerRunner := cmd.landWorkerRunner(logger)

	return <-ifrit.Invoke(landWorkerRunner).Wait()
}

func (cmd *LandWorkerCommand) landWorkerRunner(logger lager.Logger) ifrit.Runner {
	beacon := worker.NewBeacon(
		logger,
		atc.Worker{
			Name: cmd.WorkerName,
		},
		cmd.TSA.canonical(),
	)

	return ifrit.RunFunc(beacon.LandWorker)
}
