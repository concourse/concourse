package drain

import (
	"os"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/worker/beacon"
	"github.com/tedsuo/ifrit"
)

type Runner struct {
	Logger lager.Logger
	Beacon beacon.BeaconClient
	Runner ifrit.Runner
}

func (d Runner) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	proc := ifrit.Invoke(d.Runner)

	close(ready)

	retiring := false

	for {
		select {
		case sig := <-signals:
			d.Logger.Debug("received-signal", lager.Data{"signal": sig})

			if IsLand(sig) {
				d.Logger.Info("landing-worker")

				err := d.Beacon.LandWorker()
				if err != nil {
					d.Logger.Error("failed-to-land-worker", err)
				}
			} else if IsRetire(sig) {
				retiring = true

				d.Logger.Info("retiring-worker")

				err := d.Beacon.RetireWorker()
				if err != nil {
					d.Logger.Error("failed-to-retire-worker", err)
				}
			} else if IsStop(sig) {
				d.Logger.Info("stopping")

				if retiring {
					d.Logger.Info("deleting-worker")

					err := d.Beacon.DeleteWorker()
					if err != nil {
						d.Logger.Error("failed-to-delete-worker", err)
					}
				}

				proc.Signal(sig)
			} else {
				proc.Signal(sig)
			}

		case err := <-proc.Wait():
			return err
		}
	}
}
