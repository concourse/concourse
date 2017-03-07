package worker

import (
	"errors"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/dbng"
)

//go:generate counterfeiter . WorkerProvider

type WorkerProvider interface {
	RunningWorkers() ([]Worker, error)
	GetWorker(string) (Worker, bool, error)
	FindContainerForIdentifier(Identifier) (db.SavedContainer, bool, error)
	GetContainer(string) (db.SavedContainer, bool, error)
}

var (
	ErrNoWorkers     = errors.New("no workers")
	ErrMissingWorker = errors.New("worker for container is missing")
)

type NoCompatibleWorkersError struct {
	Spec    WorkerSpec
	Workers []Worker
}

func (err NoCompatibleWorkersError) Error() string {
	availableWorkers := ""
	for _, worker := range err.Workers {
		availableWorkers += "\n  - " + worker.Description()
	}

	return fmt.Sprintf(
		"no workers satisfying: %s\n\navailable workers: %s",
		err.Spec.Description(),
		availableWorkers,
	)
}

type pool struct {
	provider WorkerProvider

	rand *rand.Rand
}

func NewPool(provider WorkerProvider) Client {
	return &pool{
		provider: provider,
		rand:     rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func shuffleWorkers(slice []Worker) {
	for i := range slice {
		j := rand.Intn(i + 1)
		slice[i], slice[j] = slice[j], slice[i]
	}
}

func (pool *pool) RunningWorkers() ([]Worker, error) {
	return pool.provider.RunningWorkers()
}

func (pool *pool) GetWorker(workerName string) (Worker, error) {
	worker, found, err := pool.provider.GetWorker(workerName)
	if err != nil {
		return nil, err
	}

	if !found {
		return nil, ErrNoWorkers
	}

	return worker, nil
}

func (pool *pool) AllSatisfying(spec WorkerSpec, resourceTypes atc.VersionedResourceTypes) ([]Worker, error) {
	workers, err := pool.provider.RunningWorkers()
	if err != nil {
		return nil, err
	}

	if len(workers) == 0 {
		return nil, ErrNoWorkers
	}

	compatibleTeamWorkers := []Worker{}
	compatibleGeneralWorkers := []Worker{}
	for _, worker := range workers {
		satisfyingWorker, err := worker.Satisfying(spec, resourceTypes)
		if err == nil {
			if worker.IsOwnedByTeam() {
				compatibleTeamWorkers = append(compatibleTeamWorkers, satisfyingWorker)
			} else {
				compatibleGeneralWorkers = append(compatibleGeneralWorkers, satisfyingWorker)
			}
		}
	}

	if len(compatibleTeamWorkers) != 0 {
		shuffleWorkers(compatibleTeamWorkers)
		return compatibleTeamWorkers, nil
	}

	if len(compatibleGeneralWorkers) != 0 {
		shuffleWorkers(compatibleGeneralWorkers)
		return compatibleGeneralWorkers, nil
	}

	return nil, NoCompatibleWorkersError{
		Spec:    spec,
		Workers: workers,
	}
}

func (pool *pool) Satisfying(spec WorkerSpec, resourceTypes atc.VersionedResourceTypes) (Worker, error) {
	compatibleWorkers, err := pool.AllSatisfying(spec, resourceTypes)
	if err != nil {
		return nil, err
	}
	randomWorker := compatibleWorkers[pool.rand.Intn(len(compatibleWorkers))]
	return randomWorker, nil
}

func (pool *pool) FindOrCreateBuildContainer(logger lager.Logger, signals <-chan os.Signal, delegate ImageFetchingDelegate, id Identifier, metadata Metadata, spec ContainerSpec, resourceTypes atc.VersionedResourceTypes, outputPaths map[string]string) (Container, error) {
	container, found, err := pool.FindContainerForIdentifier(logger, id)
	if err != nil {
		return nil, err
	}
	if found {
		return container, nil
	}

	worker, err := pool.Satisfying(spec.WorkerSpec(), resourceTypes)
	if err != nil {
		return nil, err
	}

	return worker.FindOrCreateBuildContainer(logger, signals, delegate, id, metadata, spec, resourceTypes, outputPaths)
}

func (pool *pool) CreateResourceGetContainer(
	logger lager.Logger,
	resourceUser dbng.ResourceUser,
	cancel <-chan os.Signal,
	delegate ImageFetchingDelegate,
	id Identifier,
	metadata Metadata,
	spec ContainerSpec,
	resourceTypes atc.VersionedResourceTypes,
	outputPaths map[string]string,
	resourceType string,
	version atc.Version,
	source atc.Source,
	params atc.Params,
) (Container, error) {
	container, found, err := pool.FindContainerForIdentifier(logger, id)
	if err != nil {
		return nil, err
	}
	if found {
		return container, nil
	}

	worker, err := pool.Satisfying(spec.WorkerSpec(), resourceTypes)
	if err != nil {
		return nil, err
	}

	return worker.CreateResourceGetContainer(
		logger,
		resourceUser,
		cancel,
		delegate,
		id,
		metadata,
		spec,
		resourceTypes,
		outputPaths,
		resourceType,
		version,
		source,
		params,
	)
}

func (pool *pool) FindOrCreateResourceCheckContainer(
	logger lager.Logger,
	resourceUser dbng.ResourceUser,
	cancel <-chan os.Signal,
	delegate ImageFetchingDelegate,
	id Identifier,
	metadata Metadata,
	spec ContainerSpec,
	resourceTypes atc.VersionedResourceTypes,
	resourceType string,
	source atc.Source,
) (Container, error) {
	container, found, err := pool.FindContainerForIdentifier(logger, id)
	if err != nil {
		return nil, err
	}
	if found {
		return container, nil
	}

	worker, err := pool.Satisfying(spec.WorkerSpec(), resourceTypes)
	if err != nil {
		return nil, err
	}

	return worker.FindOrCreateResourceCheckContainer(
		logger,
		resourceUser,
		cancel,
		delegate,
		id,
		metadata,
		spec,
		resourceTypes,
		resourceType,
		source,
	)
}

func (pool *pool) FindContainerForIdentifier(logger lager.Logger, id Identifier) (Container, bool, error) {
	cLog := logger.Session("find-container-for-identifier", lager.Data{
		"id": id,
	})

	containerInfo, found, err := pool.provider.FindContainerForIdentifier(id)
	if err != nil {
		cLog.Error("failed-to-find", err)
		return nil, false, err
	}

	if !found {
		cLog.Info("not-found")
		return nil, found, nil
	}

	cLog = cLog.WithData(lager.Data{
		"container": containerInfo.Handle,
		"worker":    containerInfo.WorkerName,
	})

	worker, found, err := pool.provider.GetWorker(containerInfo.WorkerName)
	if err != nil {
		cLog.Error("failed-to-get-worker", err)
		return nil, false, err
	}

	if !found {
		cLog.Info("worker-is-missing")
		return nil, false, ErrMissingWorker
	}

	container, found, err := worker.FindContainerByHandle(logger, containerInfo.Handle, containerInfo.TeamID)
	if err != nil {
		cLog.Error("failed-to-find-container-by-handle", err)
		return nil, false, err
	}

	if !found {
		cLog.Info("missing-on-worker")
		return nil, false, err
	}

	return container, true, nil
}

func (pool *pool) FindContainerByHandle(logger lager.Logger, handle string, teamID int) (Container, bool, error) {
	cLog := logger.Session("find-container-by-handle", lager.Data{
		"container": handle,
	})

	containerInfo, found, err := pool.provider.GetContainer(handle)
	if err != nil {
		cLog.Error("failed-to-get-container", err)
		return nil, false, err
	}

	if !found {
		cLog.Info("container-not-found")
		return nil, false, nil
	}

	cLog = cLog.WithData(lager.Data{
		"worker": containerInfo.WorkerName,
	})

	worker, found, err := pool.provider.GetWorker(containerInfo.WorkerName)
	if err != nil {
		cLog.Error("failed-to-get-worker", err)
		return nil, false, err
	}

	if !found {
		cLog.Info("worker-is-missing")
		return nil, false, ErrMissingWorker
	}

	container, found, err := worker.FindContainerByHandle(logger, handle, teamID)
	if err != nil {
		cLog.Error("failed-to-find-container-by-handle", err)
		return nil, false, err
	}

	if !found {
		cLog.Info("missing-on-worker")
		return nil, false, nil
	}

	return container, true, nil
}

func (*pool) ValidateResourceCheckVersion(container db.SavedContainer) (bool, error) {
	return false, errors.New("ValidateResourceCheckVersion not implemented for pool")
}

func (*pool) FindResourceTypeByPath(string) (atc.WorkerResourceType, bool) {
	return atc.WorkerResourceType{}, false
}

func (*pool) CreateVolumeForResourceCache(lager.Logger, VolumeSpec, *dbng.UsedResourceCache) (Volume, error) {
	return nil, errors.New("CreateVolumeForResourceCache not implemented for pool")
}

func (*pool) FindInitializedVolumeForResourceCache(logger lager.Logger, resourceCache *dbng.UsedResourceCache) (Volume, bool, error) {
	return nil, false, errors.New("FindInitializedVolumeForResourceCache not implemented for pool")
}

func (*pool) LookupVolume(lager.Logger, string) (Volume, bool, error) {
	return nil, false, errors.New("LookupVolume not implemented for pool")
}

func (pool *pool) findCompatibleWorker(
	containerSpec ContainerSpec,
	resourceTypes atc.VersionedResourceTypes,
	sources map[string]ArtifactSource,
) (Worker, []VolumeMount, []string, error) {
	compatibleWorkers, err := pool.AllSatisfying(containerSpec.WorkerSpec(), resourceTypes)
	if err != nil {
		return nil, nil, nil, err
	}

	// find the worker with the most volumes
	mounts := []VolumeMount{}
	missingSources := []string{}
	var chosenWorker Worker

	// for each worker that matches tags, platform, etc -- what is the etc?
	for _, w := range compatibleWorkers {
		candidateMounts := []VolumeMount{}
		missing := []string{}

		for name, source := range sources {
			// look at all the inputs/outputs we're looking for
			ourVolume, found, err := source.VolumeOn(w)
			if err != nil {
				return nil, nil, nil, err
			}

			if found {
				candidateMounts = append(candidateMounts, VolumeMount{
					Volume:    ourVolume,
					MountPath: resourcesDir("put/" + name),
				})
			} else {
				missing = append(missing, name)
			}
		}

		if len(candidateMounts) >= len(mounts) {
			mounts = candidateMounts
			missingSources = missing
			chosenWorker = w
		}
	}

	return chosenWorker, mounts, missingSources, nil
}

func resourcesDir(suffix string) string {
	return filepath.Join("/tmp", "build", suffix)
}
