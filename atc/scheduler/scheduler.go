package scheduler

import (
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/algorithm"
	"github.com/concourse/concourse/atc/scheduler/inputmapper"
)

type Scheduler struct {
	Pipeline     db.Pipeline
	InputMapper  inputmapper.InputMapper
	BuildStarter BuildStarter
}

func (s *Scheduler) Schedule(
	logger lager.Logger,
	versions *algorithm.VersionsDB,
	job db.Job,
	resources db.Resources,
	resourceTypes atc.VersionedResourceTypes,
) (map[string]time.Duration, error) {
	jobSchedulingTime := map[string]time.Duration{}

	jStart := time.Now()
	err := s.ensurePendingBuildExists(logger, versions, job, resources)
	jobSchedulingTime[job.Name()] = time.Since(jStart)

	if err != nil {
		return jobSchedulingTime, err
	}

	nextPendingBuilds, err := job.GetPendingBuilds()
	if err != nil {
		logger.Error("failed-to-get-all-next-pending-builds", err)
		return jobSchedulingTime, err
	}

	err = s.BuildStarter.TryStartPendingBuildsForJob(logger, job, resources, resourceTypes, nextPendingBuilds)
	jobSchedulingTime[job.Name()] = jobSchedulingTime[job.Name()] + time.Since(jStart)

	if err != nil {
		return jobSchedulingTime, err
	}

	return jobSchedulingTime, nil
}

func (s *Scheduler) ensurePendingBuildExists(
	logger lager.Logger,
	versions *algorithm.VersionsDB,
	job db.Job,
	resources db.Resources,
) error {
	inputMapping, err := s.InputMapper.SaveNextInputMapping(logger, versions, job, resources)
	if err != nil {
		return err
	}

	for _, inputConfig := range job.Config().Inputs() {
		inputSource, ok := inputMapping[inputConfig.Name]

		//trigger: true, and the version has not been used
		if ok && inputSource.InputVersion.FirstOccurrence && inputConfig.Trigger {
			err := job.EnsurePendingBuildExists()
			if err != nil {
				logger.Error("failed-to-ensure-pending-build-exists", err)
				return err
			}

			break
		}
	}

	return nil
}
