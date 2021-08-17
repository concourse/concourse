package driver

import (
	"os"

	"github.com/concourse/baggageclaim/volume"
)

type NaiveDriver struct{}

func (driver *NaiveDriver) CreateVolume(vol volume.FilesystemInitVolume) error {
	return os.Mkdir(vol.DataPath(), 0755)
}

func (driver *NaiveDriver) DestroyVolume(vol volume.FilesystemVolume) error {
	return os.RemoveAll(vol.DataPath())
}

func (driver *NaiveDriver) Recover(volume.Filesystem) error {
	// nothing to do
	return nil
}
