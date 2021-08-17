// +build !linux

package baggageclaimcmd

import (
	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/worker/baggageclaim/volume"
	"github.com/concourse/concourse/worker/baggageclaim/volume/driver"
)

func (cmd *BaggageclaimCommand) driver(logger lager.Logger) (volume.Driver, error) {
	return &driver.NaiveDriver{}, nil
}
