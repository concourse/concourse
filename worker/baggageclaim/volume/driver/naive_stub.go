// +build !windows

package driver

import (
	"os/exec"

	"github.com/concourse/concourse/worker/baggageclaim/volume"
)

func (driver *NaiveDriver) CreateCopyOnWriteLayer(
	childVol volume.FilesystemInitVolume,
	parentVol volume.FilesystemLiveVolume,
) error {
	return exec.Command("cp", "-rp", parentVol.DataPath(), childVol.DataPath()).Run()
}
