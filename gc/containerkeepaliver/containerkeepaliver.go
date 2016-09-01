package containerkeepaliver

import (
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/worker"
)

type ContainerKeepAliver interface {
	Run() error
}

//go:generate counterfeiter . ContainerKeepAliverDB

type ContainerKeepAliverDB interface {
	FindJobIDForBuild(buildID int) (int, bool, error)
	FindLatestSuccessfulBuildsPerJob() (map[int]int, error)
	FindJobContainersFromUnsuccessfulBuilds() ([]db.SavedContainer, error)
	UpdateExpiresAtOnContainer(handle string, ttl time.Duration) error
	GetAllPipelines() ([]db.SavedPipeline, error)
}

type containerKeepAliver struct {
	logger       lager.Logger
	workerClient worker.Client
	db           ContainerKeepAliverDB
}

func NewContainerKeepAliver(
	logger lager.Logger,
	workerClient worker.Client,
	db ContainerKeepAliverDB,
) ContainerKeepAliver {
	return &containerKeepAliver{
		logger:       logger,
		workerClient: workerClient,
		db:           db,
	}
}

func (cr *containerKeepAliver) Run() error {
	failedContainers, err := cr.db.FindJobContainersFromUnsuccessfulBuilds()
	if err != nil {
		cr.logger.Error("failed-to-find-unsuccessful-containers", err)
		return err
	}

	if len(failedContainers) == 0 {
		return nil
	}

	latestSuccessfulBuilds, err := cr.db.FindLatestSuccessfulBuildsPerJob()
	if err != nil {
		cr.logger.Error("failed-to-find-successful-containers", err)
	}

	// if the latest build failed update its ttl, allow everything else expire

	failedJobContainerMap := cr.buildFailedMap(failedContainers)

	for jobID, jobContainers := range failedJobContainerMap {
		maxFailedBuildID := -1
		for _, jobContainer := range jobContainers {
			if jobContainer.BuildID > maxFailedBuildID {
				maxFailedBuildID = jobContainer.BuildID
			}
		}

		for _, jobContainer := range jobContainers {
			maxSuccessfulBuildID := latestSuccessfulBuilds[jobID]

			if maxSuccessfulBuildID > maxFailedBuildID || maxFailedBuildID > jobContainer.BuildID {
			} else {
				cr.keepAlive(jobContainer.Handle)
			}
		}
	}

	return nil
}

func (cr *containerKeepAliver) buildFailedMap(containers []db.SavedContainer) map[int][]db.SavedContainer {
	var jobContainerMap map[int][]db.SavedContainer
	jobContainerMap = make(map[int][]db.SavedContainer)

	savedPipelines, err := cr.db.GetAllPipelines()
	if err != nil {
		cr.logger.Error("failed-to-get-all-pipelines", err)
		return nil
	}

	pipelinesByIDs := map[int]db.SavedPipeline{}
	for _, savedPipeline := range savedPipelines {
		pipelinesByIDs[savedPipeline.ID] = savedPipeline
	}

	for _, container := range containers {
		savedPipeline, ok := pipelinesByIDs[container.PipelineID]
		if !ok {
			cr.logger.Error("failed-to-find-pipeline-for-build", err, lager.Data{"build-id": container.BuildID})
			continue
		}

		jobExpired := true
		for _, jobConfig := range savedPipeline.Config.Jobs {
			if jobConfig.Name == container.JobName {
				jobExpired = false
				break
			}
		}

		if jobExpired {
			cr.logger.Debug("job-expired", lager.Data{"build-id": container.BuildID})
			continue
		}

		buildID := container.BuildID
		jobID, found, err := cr.db.FindJobIDForBuild(buildID)
		if err != nil || !found {
			cr.logger.Error("find-job-id-for-build", err, lager.Data{"build-id": buildID, "found": found})
			continue
		}

		jobContainers := jobContainerMap[jobID]
		if jobContainers == nil {
			jobContainerMap[jobID] = []db.SavedContainer{container}
		} else {
			jobContainers = append(jobContainers, container)
			jobContainerMap[jobID] = jobContainers
		}
	}

	return jobContainerMap
}

func (cr *containerKeepAliver) keepAlive(handle string) {
	cr.logger.Debug("keeping alive container", lager.Data{"handle": handle})
	workerContainer, found, err := cr.workerClient.LookupContainer(cr.logger, handle)
	if err != nil {
		cr.logger.Error("failed-to-keep-alive-container", err, lager.Data{"handle": handle})
	}

	if found {
		workerContainer.Release(nil)
	}
}
