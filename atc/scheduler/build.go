package scheduler

import (
	"context"
	"fmt"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/db"
)

type manualTriggerBuild struct {
	db.Build

	job       db.Job
	jobInputs db.InputConfigs

	algorithm Algorithm
}

func (m *manualTriggerBuild) IsReadyToDetermineInputs(logger lager.Logger) (bool, error) {
	return m.ResourcesChecked()
}

func (m *manualTriggerBuild) BuildInputs(ctx context.Context) ([]db.BuildInput, bool, error) {
	inputMapping, resolved, hasNextInputs, err := m.algorithm.Compute(ctx, m.job, m.jobInputs)
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

func (s *schedulerBuild) IsReadyToDetermineInputs(logger lager.Logger) (bool, error) {
	return true, nil
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

func (r *rerunBuild) IsReadyToDetermineInputs(logger lager.Logger) (bool, error) {
	return true, nil
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
