package scheduler

import (
	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
)

//go:generate counterfeiter . Algorithm

type Algorithm interface {
	Compute(*db.VersionsDB, db.Job, db.Resources) (db.InputMapping, bool, error)
}

type Scheduler struct {
	Algorithm    Algorithm
	BuildStarter BuildStarter
}

func (s *Scheduler) Schedule(
	logger lager.Logger,
	versions *db.VersionsDB,
	job db.Job,
	resources db.Resources,
) error {
	inputMapping, resolved, err := s.Algorithm.Compute(versions, job, resources)
	if err != nil {
		return err
	}

	err = job.SaveNextInputMapping(inputMapping, resolved)
	if err != nil {
		logger.Error("failed-to-save-next-input-mapping", err)
		return err
	}

	err = s.ensurePendingBuildExists(logger, job, resources)
	if err != nil {
		return err
	}

	err = s.BuildStarter.TryStartPendingBuildsForJob(logger, job, resources)
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
	buildInputs, found, err := job.GetFullNextBuildInputs()
	if err != nil {
		logger.Error("failed-to-fetch-next-build-inputs", err)
		return err
	}

	if !found {
		// XXX: better info log pls
		logger.Info("next-build-inputs-not-found")
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
