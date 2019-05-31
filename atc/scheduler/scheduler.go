package scheduler

import (
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/v5/atc"
	"github.com/concourse/concourse/v5/atc/db"
	"github.com/concourse/concourse/v5/atc/db/algorithm"
	"github.com/concourse/concourse/v5/atc/scheduler/inputmapper"
)

type Scheduler struct {
	Pipeline     db.Pipeline
	InputMapper  inputmapper.InputMapper
	BuildStarter BuildStarter
}

func (s *Scheduler) Schedule(
	logger lager.Logger,
	versions *algorithm.VersionsDB,
	jobs []db.Job,
	resources db.Resources,
	resourceTypes atc.VersionedResourceTypes,
) (map[string]time.Duration, error) {
	jobSchedulingTime := map[string]time.Duration{}

	for _, job := range jobs {
		jStart := time.Now()
		err := s.ensurePendingBuildExists(logger, versions, job, resources)
		jobSchedulingTime[job.Name()] = time.Since(jStart)

		if err != nil {
			return jobSchedulingTime, err
		}
	}

	nextPendingBuilds, err := s.Pipeline.GetAllPendingBuilds()
	if err != nil {
		logger.Error("failed-to-get-all-next-pending-builds", err)
		return jobSchedulingTime, err
	}

	for _, job := range jobs {
		jStart := time.Now()
		nextPendingBuildsForJob, ok := nextPendingBuilds[job.Name()]
		if !ok {
			continue
		}

		err := s.BuildStarter.TryStartPendingBuildsForJob(logger, job, resources, resourceTypes, nextPendingBuildsForJob)
		jobSchedulingTime[job.Name()] = jobSchedulingTime[job.Name()] + time.Since(jStart)

		if err != nil {
			return jobSchedulingTime, err
		}
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

	var hasNewInputs bool
	for _, inputConfig := range job.Config().Inputs() {
		inputVersion, ok := inputMapping[inputConfig.Name]

		//trigger: true, and the version has not been used
		if ok && inputVersion.FirstOccurrence {
			hasNewInputs = true
			if inputConfig.Trigger {
				err := job.EnsurePendingBuildExists()
				if err != nil {
					logger.Error("failed-to-ensure-pending-build-exists", err)
					return err
				}

				break
			}
		}
	}

	if hasNewInputs != job.HasNewInputs() {
		if err := job.SetHasNewInputs(hasNewInputs); err != nil {
			return err
		}
	}

	return nil
}
