package worker

import (
	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/lager"
)

type CertsVolumeMount struct {
	Logger lager.Logger
}

func (s *CertsVolumeMount) VolumeOn(worker Worker) (garden.BindMount, bool, error) {
	volume, found, err := worker.CertsVolume(s.Logger.Session("worker-certs-volume"))
	if err != nil {
		return garden.BindMount{}, false, err
	}

	if !found {
		return garden.BindMount{}, false, err
	}

	return garden.BindMount{
		SrcPath: volume.Path(),
		DstPath: "/etc/ssl/certs",
		Mode:    garden.BindMountModeRO,
	}, true, nil
}
