package containerreaper

import (
	"time"

	"github.com/concourse/atc/db"
	"github.com/concourse/atc/worker"
	"github.com/pivotal-golang/lager"
)

type ContainerReaper interface {
	Run() error
}

//go:generate counterfeiter . ContainerReaperDB

type ContainerReaperDB interface {
	FindJobIDForBuild(buildID int) (int, bool, error)
	GetContainersWithInfiniteTTL() ([]db.SavedContainer, error)
	UpdateExpiresAtOnContainer(handle string, ttl time.Duration) error
}

type containerReaper struct {
	logger            lager.Logger
	db                ContainerReaperDB
	pipelineDBFactory db.PipelineDBFactory
	batchSize         int
}

func NewContainerReaper(
	logger lager.Logger,
	db ContainerReaperDB,
	pipelineDBFactory db.PipelineDBFactory,
	batchSize int,
) ContainerReaper {
	return &containerReaper{
		logger:            logger,
		db:                db,
		pipelineDBFactory: pipelineDBFactory,
		batchSize:         batchSize,
	}
}

func (cr *containerReaper) Run() error {
	// for all the containers associated with builds that succeeded
	//		set expiring TTL => 5 min
	// update containers left join builds set c.ttl = 5 minutes where b.status is not success

	// for all containers associated with builds that did fail
	// 		set expiring TTL on pre-latest ones for each job

	containers, err := cr.db.GetContainersWithInfiniteTTL()
	if err != nil {
		return err
	}

	var jobContainerMap map[int][]db.SavedContainer
	jobContainerMap = make(map[int][]db.SavedContainer)

	for _, container := range containers {
		buildID := container.BuildID

		// TODO: if the container's job no longer exists in the configuration, expire it
		jobID, found, err := cr.db.FindJobIDForBuild(buildID)
		if err != nil {
			cr.logger.Error("find-job-id-for-build", err, lager.Data{"build-id": buildID})
		}
		if !found {
			cr.logger.Error("unable-to-find-job-id-for-build", nil, lager.Data{"build-id": buildID})
		}

		jobContainers := jobContainerMap[jobID]
		if jobContainers == nil {
			jobContainerMap[jobID] = []db.SavedContainer{container}
		} else {
			jobContainers = append(jobContainers, container)
			jobContainerMap[jobID] = jobContainers
		}
	}

	for _, jobContainers := range jobContainerMap {
		maxBuildID := -1
		for _, jobContainer := range jobContainers {
			if jobContainer.BuildID > maxBuildID {
				maxBuildID = jobContainer.BuildID
			}
		}

		for _, jobContainer := range jobContainers {
			if jobContainer.BuildID < maxBuildID {
				handle := jobContainer.Container.Handle
				err := cr.db.UpdateExpiresAtOnContainer(handle, worker.ContainerTTL)
				if err != nil {
					cr.logger.Error("set-expiring-ttl", err, lager.Data{"container-handle": handle})
				}
			}
		}
	}

	return nil
}
