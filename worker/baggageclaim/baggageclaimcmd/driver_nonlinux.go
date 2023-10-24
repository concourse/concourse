//go:build !linux
// +build !linux

package baggageclaimcmd

import (
	"code.cloudfoundry.org/lager/v3"
	"github.com/concourse/concourse/worker/baggageclaim/volume"
	"github.com/concourse/concourse/worker/baggageclaim/volume/driver"
)

func (cmd *BaggageclaimCommand) driver(logger lager.Logger) (volume.Driver, error) {
	return &driver.NaiveDriver{}, nil
}
