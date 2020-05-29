package worker

import "code.cloudfoundry.org/garden"

type HolepunchMount struct {
	FromPath, ToPath string
}

func (m *HolepunchMount) VolumeOn(worker Worker) (garden.BindMount, bool, error) {
	return garden.BindMount{
		SrcPath: m.FromPath,
		DstPath: m.ToPath,
		Mode:    garden.BindMountModeRW,
	}, true, nil
}
