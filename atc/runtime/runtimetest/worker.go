package runtimetest

import (
	"context"
	"fmt"
	"reflect"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/dbfakes"
	"github.com/concourse/concourse/atc/runtime"
)

type WorkerContainer struct {
	*Container
	Mounts []runtime.VolumeMount
	Owner  db.ContainerOwner
	Spec   *runtime.ContainerSpec
}

type Worker struct {
	WorkerName string
	Containers []*WorkerContainer
	Volumes    []*Volume
	DBWorker_  *dbfakes.FakeWorker
}

func NewWorker(name string) *Worker {
	dbWorker := new(dbfakes.FakeWorker)
	dbWorker.NameReturns(name)
	return &Worker{WorkerName: name, DBWorker_: dbWorker}
}

func (w Worker) WithVolumes(volumes ...*Volume) *Worker {
	w2 := w
	w2.Volumes = make([]*Volume, len(w.Volumes)+len(volumes))
	copy(w2.Volumes, w.Volumes)
	copy(w2.Volumes[len(w.Volumes):], volumes)
	return &w2
}

func (w Worker) WithContainer(owner db.ContainerOwner, container *Container, mounts []runtime.VolumeMount) *Worker {
	w2 := w
	w2.Containers = make([]*WorkerContainer, len(w.Containers))
	copy(w2.Containers, w.Containers)

	workerContainer := &WorkerContainer{
		Owner:     owner,
		Container: container,
		Mounts:    mounts,
	}

	_, i, ok := w2.FindContainerByOwner(owner)
	if ok {
		w.Containers[i] = workerContainer
	} else {
		w2.Containers = append(w2.Containers, workerContainer)

	}

	return &w2
}

func (w *Worker) AddContainer(owner db.ContainerOwner, container *Container, mounts []runtime.VolumeMount) {
	*w = *w.WithContainer(owner, container, mounts)
}

func (w Worker) Name() string {
	return w.WorkerName
}

func (w Worker) CreateVolumeForArtifact(logger lager.Logger, teamID int) (runtime.Volume, db.WorkerArtifact, error) {
	panic("unimplemented")
}

func (w *Worker) FindOrCreateContainer(ctx context.Context, owner db.ContainerOwner, metadata db.ContainerMetadata, spec runtime.ContainerSpec) (runtime.Container, []runtime.VolumeMount, error) {
	c, _, ok := w.FindContainerByOwner(owner)
	if !ok {
		panic("unimplemented: runtimetest.Worker.FindOrCreateContainer can currently only find a container.\n" +
			fmt.Sprintf("missing owner: %+v", owner))
	}
	c.Spec = &spec
	return c.Container, c.Mounts, nil
}

func (w Worker) FindContainerByOwner(owner db.ContainerOwner) (*WorkerContainer, int, bool) {
	for i, c := range w.Containers {
		if reflect.DeepEqual(c.Owner, owner) {
			return c, i, true
		}
	}
	return nil, 0, false
}

func (w Worker) LookupContainer(logger lager.Logger, handle string) (runtime.Container, bool, error) {
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

func (w Worker) DBWorker() db.Worker {
	return w.DBWorker_
}
