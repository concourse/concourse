package worker

import (
	"context"
	"os"
	"sync/atomic"

	"code.cloudfoundry.org/lager"
	"github.com/tedsuo/ifrit"
)

type DrainRunner struct {
	Logger       lager.Logger
	Client       TSAClient
	Runner       ifrit.Runner
	DrainSignals <-chan os.Signal

	drained int32
}

func (d *DrainRunner) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	proc := ifrit.Background(d.Runner)

	close(ready)

	retiring := false

	ctx := context.Background()

	for {
		select {
		case sig := <-d.DrainSignals:
			atomic.StoreInt32(&d.drained, 1)

			d.Logger.Debug("received-drain-signal", lager.Data{
				"signal": sig.String(),
			})

			if isLand(sig) {
				d.Logger.Info("landing-worker")

				err := d.Client.Land(ctx)
				if err != nil {
					d.Logger.Error("failed-to-land-worker", err)
					proc.Signal(os.Interrupt)
				}
			} else if isRetire(sig) {
				retiring = true

				d.Logger.Info("retiring-worker")

				err := d.Client.Retire(ctx)
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

				err := d.Client.Delete(ctx)
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

func (d *DrainRunner) Drained() bool {
	return atomic.LoadInt32(&d.drained) == 1
}
