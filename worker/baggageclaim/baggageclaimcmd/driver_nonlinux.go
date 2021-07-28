// +build !linux

package baggageclaimcmd

import (
	"code.cloudfoundry.org/lager"
	"github.com/concourse/baggageclaim/volume"
	"github.com/concourse/baggageclaim/volume/driver"
)

func (cmd *BaggageclaimCommand) driver(logger lager.Logger) (volume.Driver, error) {
	return &driver.NaiveDriver{}, nil
}
