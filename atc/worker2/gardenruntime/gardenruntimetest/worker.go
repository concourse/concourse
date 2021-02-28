package gardenruntimetest

import (
	"strconv"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/compression"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/dbtest"
	"github.com/concourse/concourse/atc/runtime"
	"github.com/concourse/concourse/atc/worker2"
	"github.com/concourse/concourse/atc/worker2/gardenruntime"
	"github.com/concourse/concourse/atc/worker2/workertest"

	. "github.com/onsi/gomega"
)

type DBState int

const (
	_ DBState = iota
	Creating
	Created
)

type SetupFunc func(*Worker, *workertest.Scenario)

type Worker struct {
	WorkerName string
	Containers []*Container
	Volumes    []*Volume
	SetupFuncs []SetupFunc
}

func NewWorker(name string) *Worker {
	return &Worker{WorkerName: name}
}

func (w Worker) Name() string {
	return w.WorkerName
}

func (w *Worker) Setup(s *workertest.Scenario) {
	atcWorker := dbtest.BaseWorker(w.Name())
	atcWorker.ActiveContainers = len(w.Containers)
	atcWorker.ActiveVolumes = len(w.Volumes)

	s.DB.Run(s.DBBuilder.WithWorker(atcWorker))

	for _, f := range w.SetupFuncs {
		s.Run(func(s *workertest.Scenario) { f(w, s) })
	}
}

func (w Worker) Build(pool worker2.Pool, dbWorker db.Worker) runtime.Worker {
	return gardenruntime.NewWorker(
		dbWorker,
		&Garden{ContainerList: w.Containers},
		&Baggageclaim{Volumes: w.Volumes},
		pool.DB.ToGardenRuntimeDB(),
		pool,
		worker2.Streamer{Compression: compression.NewGzipCompression()},
	)
}

func (w Worker) WithGardenContainers(containers ...*Container) *Worker {
	w2 := w
	w2.Containers = make([]*Container, len(w.Containers)+len(containers))
	copy(w2.Containers, w.Containers)
	copy(w2.Containers[len(w.Containers):], containers)
	return &w2
}

func (w Worker) WithBaggageclaimVolumes(volumes ...*Volume) *Worker {
	w2 := w
	w2.Volumes = make([]*Volume, len(w.Volumes)+len(volumes))
	copy(w2.Volumes, w.Volumes)
	copy(w2.Volumes[len(w.Volumes):], volumes)
	return &w2
}

func (w Worker) WithWorkerSetup(setup ...SetupFunc) *Worker {
	w2 := w
	w2.SetupFuncs = make([]SetupFunc, len(w.SetupFuncs)+len(setup))
	copy(w2.SetupFuncs, w.SetupFuncs)
	copy(w2.SetupFuncs[len(w.SetupFuncs):], setup)
	return &w2
}

func (w Worker) WithSetup(setup ...workertest.SetupFunc) *Worker {
	workerSetup := make([]SetupFunc, len(setup))
	for i, f := range setup {
		workerSetup[i] = func(_ *Worker, s *workertest.Scenario) { f(s) }
	}
	return w.WithWorkerSetup(workerSetup...)
}

func (w Worker) WithDBContainerVolumesInState(state DBState, containerHandle string, paths ...string) *Worker {
	return w.WithSetup(func(s *workertest.Scenario) {
		containerOwner := db.NewFixedHandleContainerOwner(containerHandle)
		container := s.DB.Container(w.Name(), containerOwner).(db.CreatingContainer)

		for _, path := range paths {
			volume, err := s.DBBuilder.VolumeRepo.CreateContainerVolume(s.TeamID, w.Name(), container, path)
			Expect(err).ToNot(HaveOccurred())

			if state == Created {
				_, err := volume.Created()
				Expect(err).ToNot(HaveOccurred())
			}
		}
	})
}

func (w Worker) WithDBContainersInState(state DBState, handles ...string) *Worker {
	return w.WithSetup(func(s *workertest.Scenario) {
		for _, handle := range handles {
			owner := db.NewFixedHandleContainerOwner(handle)
			switch state {
			case Creating:
				s.DB.Run(s.DBBuilder.WithCreatingContainer(w.Name(), owner, db.ContainerMetadata{}))
			case Created:
				s.DB.Run(s.DBBuilder.WithCreatedContainer(w.Name(), owner, db.ContainerMetadata{}))
			default:
				panic("invalid state " + strconv.Itoa(int(state)))
			}
		}
	})
}

func (w Worker) WithDBVolumesInState(state DBState, handles ...string) *Worker {
	return w.WithSetup(func(s *workertest.Scenario) {
		for _, handle := range handles {
			switch state {
			case Creating:
				s.DB.Run(s.DBBuilder.WithCreatingVolume(0, w.Name(), db.VolumeTypeContainer, handle))
			case Created:
				s.DB.Run(s.DBBuilder.WithCreatedVolume(0, w.Name(), db.VolumeTypeContainer, handle))
			default:
				panic("invalid state " + strconv.Itoa(int(state)))
			}
		}
	})
}

func (w *Worker) WithCachedPaths(cachedPaths ...string) *Worker {
	return w.WithWorkerSetup(func(w *Worker, s *workertest.Scenario) {
		for _, cachePath := range cachedPaths {
			s.DB.Run(s.DBBuilder.WithTaskCacheOnWorker(s.TeamID, w.Name(), s.JobID, s.StepName, cachePath))
			cacheVolume := s.WorkerTaskCacheVolume(w.Name(), cachePath)
			w.Volumes = append(w.Volumes, NewVolume(cacheVolume.Handle()))
		}
	})
}

func (w *Worker) WithResourceCacheOnVolume(containerHandle string, volumeHandle string, resourceTypeName string) *Worker {
	return w.WithSetup(func(s *workertest.Scenario) {
		container := s.DB.Container(w.Name(), db.NewFixedHandleContainerOwner(containerHandle))
		cache, err := s.DBBuilder.ResourceCacheFactory.FindOrCreateResourceCache(
			db.ForContainer(container.ID()),
			resourceTypeName,
			atc.Version{},
			atc.Source{},
			atc.Params{},
			atc.VersionedResourceTypes{
				{
					ResourceType: atc.ResourceType{
						Name: resourceTypeName,
						Type: dbtest.BaseResourceType,
					},
				},
			},
		)
		Expect(err).ToNot(HaveOccurred())

		_, volume, err := s.DBBuilder.VolumeRepo.FindVolume(volumeHandle)
		Expect(err).ToNot(HaveOccurred())

		err = volume.InitializeResourceCache(cache)
		Expect(err).ToNot(HaveOccurred())
	})
}

func (w Worker) WithContainersCreatedInDBAndGarden(containers ...*Container) *Worker {
	return w.WithGardenContainers(containers...).WithDBContainersInState(Created, containerHandles(containers)...)
}

func (w Worker) WithVolumesCreatedInDBAndBaggageclaim(volumes ...*Volume) *Worker {
	return w.WithBaggageclaimVolumes(volumes...).WithDBVolumesInState(Created, volumeHandles(volumes)...)
}

func (w Worker) WithActiveTasks(activeTasks int) *Worker {
	return w.WithSetup(func(s *workertest.Scenario) {
		worker := s.DB.Worker(w.Name())
		for i := 0; i < activeTasks; i++ {
			err := worker.IncreaseActiveTasks()
			Expect(err).ToNot(HaveOccurred())
		}
	})
}

func containerHandles(containers []*Container) []string {
	handles := make([]string, len(containers))
	for i, c := range containers {
		handles[i] = c.handle
	}
	return handles
}

func volumeHandles(volumes []*Volume) []string {
	handles := make([]string, len(volumes))
	for i, v := range volumes {
		handles[i] = v.handle
	}
	return handles
}
