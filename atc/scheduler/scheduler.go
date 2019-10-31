package scheduler

import (
	"fmt"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/scheduler/algorithm"
)

//go:generate counterfeiter . Algorithm

type Algorithm interface {
	Compute(db.Job, db.Resources, algorithm.NameToIDMap) (db.InputMapping, bool, bool, error)
}

type Scheduler struct {
	Algorithm    Algorithm
	BuildStarter BuildStarter
}

func (s *Scheduler) Schedule(
	logger lager.Logger,
	pipeline db.Pipeline,
	job db.Job,
	resources db.Resources,
	relatedJobs algorithm.NameToIDMap,
) error {
	inputMapping, resolved, runAgain, err := s.Algorithm.Compute(job, resources, relatedJobs)
	if err != nil {
		return fmt.Errorf("compute inputs: %w", err)
	}

	if runAgain {
		err = pipeline.RequestSchedule()
		if err != nil {
			return fmt.Errorf("request schedule: %w", err)
		}
	}

	err = job.SaveNextInputMapping(inputMapping, resolved)
	if err != nil {
		return fmt.Errorf("save next input mapping: %w", err)
	}

	err = s.ensurePendingBuildExists(logger, job, resources)
	if err != nil {
		return err
	}

	err = s.BuildStarter.TryStartPendingBuildsForJob(logger, pipeline, job, resources, relatedJobs)
	if err != nil {
		return err
	}

	return nil
}

func (s *Scheduler) ensurePendingBuildExists(
	logger lager.Logger,
	job db.Job,
	resources db.Resources,
) error {
	buildInputs, inputsDetermined, err := job.GetFullNextBuildInputs()
	if err != nil {
		return fmt.Errorf("get next build inputs: %w", err)
	}

	if !inputsDetermined {
		logger.Info("next-build-inputs-not-determined")
		return nil
	}

	inputMapping := map[string]db.BuildInput{}
	for _, input := range buildInputs {
		inputMapping[input.Name] = input
	}

	var hasNewInputs bool
	for _, inputConfig := range job.Config().Inputs() {
		inputSource, ok := inputMapping[inputConfig.Name]

		//trigger: true, and the version has not been used
		if ok && inputSource.FirstOccurrence {
			hasNewInputs = true
			if inputConfig.Trigger {
				err := job.EnsurePendingBuildExists()
				if err != nil {
					return fmt.Errorf("ensure pending build exists: %w", err)
				}

				break
			}
		}
	}

	if hasNewInputs != job.HasNewInputs() {
		if err := job.SetHasNewInputs(hasNewInputs); err != nil {
			return fmt.Errorf("set has new inputs: %w", err)
		}
	}

	return nil
}
