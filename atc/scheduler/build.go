package scheduler

import (
	"fmt"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/scheduler/algorithm"
)

type manualTriggerBuild struct {
	db.Build

	job           db.Job
	resources     db.Resources
	relatedJobIDs algorithm.NameToIDMap

	algorithm Algorithm
}

func (m *manualTriggerBuild) BuildInputs(logger lager.Logger) ([]db.BuildInput, bool, error) {
	for _, input := range m.job.Config().Inputs() {
		resource, found := m.resources.Lookup(input.Resource)

		if !found {
			logger.Debug("failed-to-find-resource")
			return nil, false, nil
		}

		if resource.CurrentPinnedVersion() != nil {
			continue
		}

		if m.IsNewerThanLastCheckOf(resource) {
			logger.Debug("resource-not-checked-yet")
			return nil, false, nil
		}
	}

	inputMapping, resolved, hasNextInputs, err := m.algorithm.Compute(m.job, m.resources, m.relatedJobIDs)
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

	return m.AdoptInputsAndPipes()
}

type schedulerBuild struct {
	db.Build
}

func (s *schedulerBuild) BuildInputs(logger lager.Logger) ([]db.BuildInput, bool, error) {
	return s.AdoptInputsAndPipes()
}

type rerunBuild struct {
	db.Build
}

func (r *rerunBuild) BuildInputs(logger lager.Logger) ([]db.BuildInput, bool, error) {
	return r.AdoptRerunInputsAndPipes()
}
