package atccmd

import (
	"os"

	"github.com/concourse/atc/db"

	"code.cloudfoundry.org/lager"
)

type drainer struct {
	logger lager.Logger
	drain  chan<- struct{}
	bus    db.NotificationsBus
}

func (d drainer) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	close(ready)

	<-signals

	d.bus.Notify("atc_shutdown")
	close(d.drain)

	return nil
}
