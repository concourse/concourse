package scheduler

import (
	"context"
	"fmt"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/scheduler/algorithm"
)

type manualTriggerBuild struct {
	db.Build

	job           db.Job
	jobInputs     []atc.JobInput
	resources     db.Resources
	relatedJobIDs algorithm.NameToIDMap

	algorithm Algorithm
}

func (m *manualTriggerBuild) IsReadyToDetermineInputs(logger lager.Logger) bool {
	for _, input := range m.jobInputs {
		resource, found := m.resources.Lookup(input.Resource)

		if !found {
			logger.Debug("failed-to-find-resource")
			return false
		}

		if resource.CurrentPinnedVersion() != nil {
			continue
		}

		if m.IsNewerThanLastCheckOf(resource) {
			logger.Debug("resource-not-checked-yet")
			return false
		}
	}

	return true
}

func (m *manualTriggerBuild) BuildInputs(ctx context.Context) ([]db.BuildInput, bool, error) {
	inputMapping, resolved, hasNextInputs, err := m.algorithm.Compute(ctx, m.job, m.jobInputs, m.resources, m.relatedJobIDs)
	if err != nil {
		return nil, false, fmt.Errorf("compute inputs: %w", err)
	}

	if hasNextInputs {
		err = m.job.RequestSchedule()
		if err != nil {
			return nil, false, fmt.Errorf("request schedule: %w", err)
		}
	}

	err = m.job.SaveNextInputMapping(inputMapping, resolved)
	if err != nil {
		return nil, false, fmt.Errorf("save next input mapping: %w", err)
	}

	buildInputs, satisfableInputs, err := m.AdoptInputsAndPipes()
	if err != nil {
		return nil, false, fmt.Errorf("adopt inputs and pipes: %w", err)
	}

	if !satisfableInputs {
		return nil, false, nil
	}

	return buildInputs, true, nil
}

type schedulerBuild struct {
	db.Build
}

func (s *schedulerBuild) IsReadyToDetermineInputs(logger lager.Logger) bool {
	return true
}

func (s *schedulerBuild) BuildInputs(ctx context.Context) ([]db.BuildInput, bool, error) {
	buildInputs, satisfableInputs, err := s.AdoptInputsAndPipes()
	if err != nil {
		return nil, false, fmt.Errorf("adopt inputs and pipes: %w", err)
	}

	if !satisfableInputs {
		return nil, false, nil
	}

	return buildInputs, true, nil
}

type rerunBuild struct {
	db.Build
}

func (r *rerunBuild) IsReadyToDetermineInputs(logger lager.Logger) bool {
	return true
}

func (r *rerunBuild) BuildInputs(ctx context.Context) ([]db.BuildInput, bool, error) {
	buildInputs, inputsReady, err := r.AdoptRerunInputsAndPipes()
	if err != nil {
		return nil, false, fmt.Errorf("adopt rerun inputs and pipes: %w", err)
	}

	if !inputsReady {
		return nil, false, nil
	}

	return buildInputs, true, nil
}
