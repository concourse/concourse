package worker

import (
	"context"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerctx"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/metric"
	"github.com/concourse/concourse/atc/runtime"
	"github.com/cppforlife/go-semi-semantic/version"
	"github.com/hashicorp/go-multierror"
)

var PollingInterval = 5 * time.Second

type Pool struct {
	Factory
	DB DB

	WorkerVersion version.Version

	waker chan struct{}
}

func NewPool(factory Factory, db DB, workerVersion version.Version) Pool {
	return Pool{
		Factory:       factory,
		DB:            db,
		WorkerVersion: workerVersion,

		waker: make(chan struct{}),
	}
}

type PoolCallback interface {
	WaitingForWorker(lager.Logger)
}

func (pool Pool) FindOrSelectWorker(
	ctx context.Context,
	owner db.ContainerOwner,
	containerSpec runtime.ContainerSpec,
	workerSpec Spec,
	strategy PlacementStrategy,
	callback PoolCallback,
) (runtime.Worker, error) {
	logger := lagerctx.FromContext(ctx)

	started := time.Now()
	labels := metric.StepsWaitingLabels{
		Platform:   workerSpec.Platform,
		TeamId:     strconv.Itoa(workerSpec.TeamID),
		Type:       string(containerSpec.Type),
		WorkerTags: strings.Join(workerSpec.Tags, "_"),
	}
	var worker db.Worker
	var pollingTicker *time.Ticker
	for {
		var err error
		worker, err = pool.findOrSelectWorker(logger, owner, containerSpec, workerSpec, strategy)
		if err != nil {
			return nil, err
		}
		if worker != nil {
			break
		}

		if pollingTicker == nil {
			pollingTicker = time.NewTicker(PollingInterval)
			defer pollingTicker.Stop()

			logger.Debug("waiting-for-available-worker")

			_, ok := metric.Metrics.StepsWaiting[labels]
			if !ok {
				metric.Metrics.StepsWaiting[labels] = &metric.Gauge{}
			}

			metric.Metrics.StepsWaiting[labels].Inc()
			defer metric.Metrics.StepsWaiting[labels].Dec()

			if callback != nil {
				callback.WaitingForWorker(logger)
			}
		}

		select {
		case <-ctx.Done():
			logger.Info("aborted-waiting-for-worker")
			return nil, ctx.Err()
		case <-pollingTicker.C:
		case <-pool.waker:
		}
	}

	elapsed := time.Since(started)
	metric.StepsWaitingDuration{
		Labels:   labels,
		Duration: elapsed,
	}.Emit(logger)

	return pool.Factory.NewWorker(logger, pool, worker), nil
}

func (pool Pool) findOrSelectWorker(logger lager.Logger, owner db.ContainerOwner, containerSpec runtime.ContainerSpec, workerSpec Spec, strategy PlacementStrategy) (db.Worker, error) {
	worker, compatibleWorkers, found, err := pool.findWorkerForContainer(logger, owner, workerSpec)
	if err != nil {
		return nil, err
	}
	if found {
		return worker, nil
	}
	orderedWorkers, err := strategy.Order(logger, pool, compatibleWorkers, containerSpec)
	if err != nil {
		return nil, err
	}

	var strategyError error
	for _, candidate := range orderedWorkers {
		err := strategy.Pick(logger, candidate, containerSpec)

		if err == nil {
			return candidate, nil
		}

		strategyError = multierror.Append(
			strategyError,
			fmt.Errorf("worker: %s, error: %v", candidate.Name(), err),
		)
	}

	logger.Debug("all-candidate-workers-rejected-during-selection", lager.Data{"reason": strategyError.Error()})

	return nil, nil
}

func (pool Pool) ReleaseWorker(logger lager.Logger, containerSpec runtime.ContainerSpec, worker runtime.Worker, strategy PlacementStrategy) {
	strategy.Release(logger, worker.DBWorker(), containerSpec)

	// Attempt to wake a random waiting step to see if it can be
	// scheduled on the recently released worker.
	select {
	case pool.waker <- struct{}{}:
		logger.Debug("attempted-to-wake-waiting-step")
	default:
	}
}

func (pool Pool) FindWorkerForContainer(logger lager.Logger, owner db.ContainerOwner, workerSpec Spec) (runtime.Worker, bool, error) {
	worker, _, found, err := pool.findWorkerForContainer(logger, owner, workerSpec)
	if err != nil {
		return nil, false, err
	}
	if !found {
		return nil, false, nil
	}
	return pool.Factory.NewWorker(logger, pool, worker), true, nil
}

func (pool Pool) findWorkerForContainer(logger lager.Logger, owner db.ContainerOwner, workerSpec Spec) (db.Worker, []db.Worker, bool, error) {
	workersWithContainer, err := pool.DB.WorkerFactory.FindWorkersForContainerByOwner(owner)
	if err != nil {
		return nil, nil, false, err
	}

	compatibleWorkers, err := pool.allCompatible(logger, workerSpec)
	if err != nil {
		return nil, nil, false, err
	}

	for _, w := range workersWithContainer {
		for _, c := range compatibleWorkers {
			if w.Name() == c.Name() {
				return w, compatibleWorkers, true, nil
			}
		}
	}

	return nil, compatibleWorkers, false, nil
}

func (pool Pool) FindWorker(logger lager.Logger, name string) (runtime.Worker, bool, error) {
	worker, found, err := pool.DB.WorkerFactory.GetWorker(name)
	if err != nil {
		logger.Error("failed-to-get-worker", err)
		return nil, false, err
	}
	if !found {
		logger.Info("worker-not-found", lager.Data{"worker": name})
		return nil, false, nil
	}
	return pool.NewWorker(logger, pool, worker), true, nil
}

func (pool Pool) LocateVolume(logger lager.Logger, teamID int, handle string) (runtime.Volume, runtime.Worker, bool, error) {
	logger = logger.Session("worker-for-volume", lager.Data{"handle": handle, "team-id": teamID})
	team := pool.DB.TeamFactory.GetByID(teamID)

	dbWorker, found, err := team.FindWorkerForVolume(handle)
	if err != nil {
		logger.Error("failed-to-find-worker", err)
		return nil, nil, false, err
	}
	if !found {
		return nil, nil, false, nil
	}
	if !pool.isWorkerVersionCompatible(logger, dbWorker) {
		return nil, nil, false, nil
	}

	logger = logger.WithData(lager.Data{"worker": dbWorker.Name()})
	logger.Debug("found-volume-on-worker")

	worker := pool.NewWorker(logger, pool, dbWorker)

	volume, found, err := worker.LookupVolume(logger, handle)
	if err != nil {
		logger.Error("failed-to-lookup-volume", err)
		return nil, nil, false, err
	}
	if !found {
		logger.Info("volume-disappeared-from-worker")
		return nil, nil, false, nil
	}

	return volume, worker, true, nil
}

func (pool Pool) LocateContainer(logger lager.Logger, teamID int, handle string) (runtime.Container, runtime.Worker, bool, error) {
	logger = logger.Session("worker-for-container", lager.Data{"handle": handle, "team-id": teamID})
	team := pool.DB.TeamFactory.GetByID(teamID)

	dbWorker, found, err := team.FindWorkerForContainer(handle)
	if err != nil {
		logger.Error("failed-to-find-worker", err)
		return nil, nil, false, err
	}
	if !found {
		return nil, nil, false, nil
	}
	if !pool.isWorkerVersionCompatible(logger, dbWorker) {
		return nil, nil, false, nil
	}

	logger = logger.WithData(lager.Data{"worker": dbWorker.Name()})
	logger.Debug("found-volume-on-worker")

	worker := pool.NewWorker(logger, pool, dbWorker)

	container, found, err := worker.LookupContainer(logger, handle)
	if err != nil {
		logger.Error("failed-to-lookup-container", err)
		return nil, nil, false, err
	}
	if !found {
		logger.Info("container-disappeared-from-worker")
		return nil, nil, false, nil
	}

	return container, worker, true, nil
}

func (pool Pool) CreateVolumeForArtifact(logger lager.Logger, spec Spec) (runtime.Volume, db.WorkerArtifact, error) {
	compatibleWorkers, err := pool.allCompatible(logger, spec)
	if err != nil {
		return nil, nil, err
	}

	worker := pool.Factory.NewWorker(logger, pool, compatibleWorkers[rand.Intn(len(compatibleWorkers))])
	return worker.CreateVolumeForArtifact(logger, spec.TeamID)
}

func (pool Pool) allCompatible(logger lager.Logger, spec Spec) ([]db.Worker, error) {
	workers, err := pool.DB.WorkerFactory.Workers()
	if err != nil {
		return nil, err
	}

	if len(workers) == 0 {
		return nil, ErrNoWorkers
	}

	var compatibleTeamWorkers []db.Worker
	var compatibleGeneralWorkers []db.Worker
	for _, worker := range workers {
		compatible := pool.isWorkerCompatible(logger, worker, spec)
		if compatible {
			if worker.TeamID() != 0 {
				compatibleTeamWorkers = append(compatibleTeamWorkers, worker)
			} else {
				compatibleGeneralWorkers = append(compatibleGeneralWorkers, worker)
			}
		}
	}

	if len(compatibleTeamWorkers) != 0 {
		// XXX(aoldershaw): if there is a team worker that is compatible but is
		// rejected by the strategy, shouldn't we fallback to general workers?
		return compatibleTeamWorkers, nil
	}

	if len(compatibleGeneralWorkers) != 0 {
		return compatibleGeneralWorkers, nil
	}

	return nil, NoCompatibleWorkersError{
		Spec:          spec,
		WorkerVersion: pool.WorkerVersion,
	}
}

func (pool Pool) isWorkerVersionCompatible(logger lager.Logger, dbWorker db.Worker) bool {
	workerVersion := dbWorker.Version()
	logger = logger.Session("check-version", lager.Data{
		"want-worker-version": pool.WorkerVersion.String(),
		"have-worker-version": workerVersion,
	})

	if workerVersion == nil {
		logger.Info("empty-worker-version")
		return false
	}

	v, err := version.NewVersionFromString(*workerVersion)
	if err != nil {
		logger.Error("failed-to-parse-version", err)
		return false
	}

	switch v.Release.Compare(pool.WorkerVersion.Release) {
	case 0:
		return true
	case -1:
		return false
	default:
		if v.Release.Components[0].Compare(pool.WorkerVersion.Release.Components[0]) == 0 {
			return true
		}

		return false
	}
}

func (pool Pool) isWorkerCompatible(logger lager.Logger, worker db.Worker, spec Spec) bool {
	if !pool.isWorkerVersionCompatible(logger, worker) {
		return false
	}

	if worker.TeamID() != 0 {
		if spec.TeamID != worker.TeamID() {
			return false
		}
	}

	if spec.ResourceType != "" {
		matchedType := false
		for _, t := range worker.ResourceTypes() {
			if t.Type == spec.ResourceType {
				matchedType = true
				break
			}
		}

		if !matchedType {
			return false
		}
	}

	if spec.Platform != "" {
		if spec.Platform != worker.Platform() {
			return false
		}
	}

	if !tagsMatch(worker, spec.Tags) {
		return false
	}

	return true
}

func tagsMatch(worker db.Worker, tags []string) bool {
	if len(worker.Tags()) > 0 && len(tags) == 0 {
		return false
	}

	hasTag := func(tag string) bool {
		for _, wtag := range worker.Tags() {
			if wtag == tag {
				return true
			}
		}
		return false
	}

	for _, tag := range tags {
		if !hasTag(tag) {
			return false
		}
	}
	return true
}
