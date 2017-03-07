package atccmd

import (
	"os"

	"github.com/concourse/atc/builds"
	"github.com/concourse/atc/db"

	"code.cloudfoundry.org/lager"
)

type drainer struct {
	logger  lager.Logger
	drain   chan<- struct{}
	tracker builds.BuildTracker
	bus     db.NotificationsBus
}

func (d drainer) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	close(ready)

	<-signals

	d.logger.Info("releasing-tracker")
	d.tracker.Release()
	d.logger.Info("released-tracker")

	close(d.drain)
	d.logger.Info("sending-atc-shudown-message")
	d.bus.Notify("atc_shutdown")

	return nil
}
