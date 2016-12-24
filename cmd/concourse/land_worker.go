package main

import (
	"os"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/tedsuo/ifrit"
)

type BeaconConfigRequired struct {
	Host             string   `long:"host" required:"true" default:"127.0.0.1" description:"TSA host to negotiate the worker draining through."`
	Port             int      `long:"port" required:"true" default:"2222" description:"TSA port to connect to."`
	PublicKey        FileFlag `long:"public-key" required:"true" description:"File containing a public key to expect from the TSA."`
	WorkerPrivateKey FileFlag `long:"worker-private-key" required:"true" description:"File containing the private key to use when authenticating to the TSA."`
}

func (b BeaconConfigRequired) canonical() BeaconConfig {
	return BeaconConfig{
		Host:             b.Host,
		Port:             b.Port,
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
	beacon := Beacon{
		Logger: logger,
		Config: cmd.TSA.canonical(),
	}

	beacon.Worker = atc.Worker{
		Name: cmd.WorkerName,
	}

	return ifrit.RunFunc(beacon.LandWorker)
}
