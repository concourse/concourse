package runtimetest

import (
	"context"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/runtime"
)

type Worker struct {
	WorkerName string
	Volumes    []*Volume
}

func NewWorker(name string) Worker {
	return Worker{WorkerName: name}
}

func (w Worker) WithVolumes(volumes ...*Volume) Worker {
	w2 := w
	w2.Volumes = make([]*Volume, len(w.Volumes)+len(volumes))
	copy(w2.Volumes, w.Volumes)
	copy(w2.Volumes[len(w.Volumes):], volumes)
	return w2
}

func (w Worker) Name() string {
	return w.WorkerName
}

func (w Worker) FindOrCreateContainer(ctx context.Context, owner db.ContainerOwner, metadata db.ContainerMetadata, spec runtime.ContainerSpec) (runtime.Container, []runtime.VolumeMount, error) {
	panic("unimplemented")
}

func (w Worker) LookupVolume(logger lager.Logger, handle string) (runtime.Volume, bool, error) {
	for _, volume := range w.Volumes {
		if volume.Handle() == handle {
			return volume, true, nil
		}
	}
	return nil, false, nil
}
