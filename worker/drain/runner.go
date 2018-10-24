package drain

import (
	"os"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/worker/beacon"
	"github.com/tedsuo/ifrit"
)

type Runner struct {
	Logger       lager.Logger
	Beacon       beacon.BeaconClient
	Runner       ifrit.Runner
	DrainSignals <-chan os.Signal

	drained bool
}

func (d *Runner) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	proc := ifrit.Background(d.Runner)

	close(ready)

	retiring := false

	for {
		select {
		case sig := <-d.DrainSignals:
			d.drained = true

			d.Logger.Debug("received-drain-signal", lager.Data{
				"signal": sig.String(),
			})

			if IsLand(sig) {
				d.Logger.Info("landing-worker")

				err := d.Beacon.LandWorker()
				if err != nil {
					d.Logger.Error("failed-to-land-worker", err)
					proc.Signal(os.Interrupt)
				}
			} else if IsRetire(sig) {
				retiring = true

				d.Logger.Info("retiring-worker")

				err := d.Beacon.RetireWorker()
				if err != nil {
					d.Logger.Error("failed-to-retire-worker", err)
					proc.Signal(os.Interrupt)
				}
			}

		case sig := <-signals:
			d.Logger.Debug("received-shutdown-signal", lager.Data{
				"signal": sig.String(),
			})

			if retiring {
				d.Logger.Info("deleting-worker")

				err := d.Beacon.DeleteWorker()
				if err != nil {
					d.Logger.Error("failed-to-delete-worker", err)
				}
			}

			d.Logger.Info("forwarding-signal")

			proc.Signal(sig)

		case err := <-proc.Wait():
			return err
		}
	}
}

func (d *Runner) Drained() bool {
	return d.drained
}
