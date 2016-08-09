package main

import (
	"os"
	"path/filepath"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/baggageclaim/baggageclaimcmd"
	"github.com/tedsuo/ifrit"
)

func (cmd *WorkerCommand) naiveBaggageclaimRunner(logger lager.Logger) (ifrit.Runner, error) {
	volumesDir := filepath.Join(cmd.WorkDir, "volumes")

	err := os.MkdirAll(volumesDir, 0755)
	if err != nil {
		return nil, err
	}

	cmd.Baggageclaim.Metrics = cmd.Metrics
	cmd.Baggageclaim.VolumesDir = baggageclaimcmd.DirFlag(volumesDir)

	return cmd.Baggageclaim.Runner(nil)
}
