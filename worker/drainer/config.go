package drainer

import (
	"os"
	"time"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/worker"
	"github.com/concourse/concourse/worker/beacon"
	"github.com/concourse/concourse/worker/tsa"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
)

type Config struct {
	WorkerName    string         `long:"name" required:"true" description:"The name of the worker you wish to drain."`
	BeaconPidFile string         `long:"beacon-pid-file" description:"Path to beacon pid file."`
	IsShutdown    bool           `long:"shutdown" description:"Whether worker is about to shutdown."`
	Timeout       *time.Duration `long:"timeout" description:"Maximum time to wait for draining to finish."`
	TSAConfig     tsa.Config     `group:"TSA Configuration" namespace:"tsa" required:"true"`
}

func (cmd *Config) Execute(args []string) error {
	logger := lager.NewLogger("worker")
	logger.RegisterSink(lager.NewWriterSink(os.Stdout, lager.DEBUG))

	logger = logger.Session("drain")

	logger.Debug("running-drain", lager.Data{"shutdown": cmd.IsShutdown})

	beacon := worker.NewBeacon(
		logger,
		atc.Worker{
			Name: cmd.WorkerName,
		},
		beacon.Config{
			TSAConfig: cmd.TSAConfig,
		},
	)

	// drain commands need not keep the connection alive since it is just one off commands to TSA API
	beacon.DisableKeepAlive()

	drainer := &Drainer{
		BeaconClient:             beacon,
		IsShutdown:               cmd.IsShutdown,
		WatchProcess:             NewBeaconWatchProcess(cmd.BeaconPidFile),
		CheckProcessInterval:     time.Second,
		NumProcessChecksPerCycle: 15,
		Clock:   clock.NewClock(),
		Timeout: cmd.Timeout,
	}

	return drainer.Drain(logger)
}
