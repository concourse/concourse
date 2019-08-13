package scheduler

import (
	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/db"
)

type manualTriggerBuild struct {
	db.Build

	pipeline  db.Pipeline
	job       db.Job
	resources db.Resources

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
			return nil, false, nil
		}
	}

	versions, err := m.pipeline.LoadVersionsDB()
	if err != nil {
		logger.Error("failed-to-load-versions-db", err)
		return nil, false, err
	}

	inputMapping, resolved, err := m.algorithm.Compute(versions, m.job, m.resources)
	if err != nil {
		return nil, false, err
	}

	err = m.job.SaveNextInputMapping(inputMapping, resolved)
	if err != nil {
		logger.Error("failed-to-save-next-input-mapping", err)
		return nil, false, err
	}

	return m.AdoptInputsAndPipes()
}

type schedulerBuild struct {
	db.Build
}

func (s *schedulerBuild) BuildInputs(logger lager.Logger) ([]db.BuildInput, bool, error) {
	return s.AdoptInputsAndPipes()
}

type retriggerBuild struct {
	db.Build
}

func (r *retriggerBuild) BuildInputs(logger lager.Logger) ([]db.BuildInput, bool, error) {
	return r.AdoptRetriggerInputsAndPipes()
}
