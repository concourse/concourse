package scheduler

import (
	"context"
	"encoding/json"
	"fmt"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/tracing"
)

//go:generate counterfeiter . Algorithm

type Algorithm interface {
	Compute(
		context.Context,
		db.Job,
		db.InputConfigs,
	) (db.InputMapping, bool, bool, error)
}

type Scheduler struct {
	Algorithm    Algorithm
	BuildStarter BuildStarter
}

func (s *Scheduler) Schedule(
	ctx context.Context,
	logger lager.Logger,
	job db.SchedulerJob,
) (bool, error) {
	jobInputs, err := job.AlgorithmInputs()
	if err != nil {
		return false, fmt.Errorf("inputs: %w", err)
	}

	inputMapping, resolved, runAgain, err := s.Algorithm.Compute(ctx, job, jobInputs)
	if err != nil {
		return false, fmt.Errorf("compute inputs: %w", err)
	}

	if runAgain {
		err = job.RequestSchedule()
		if err != nil {
			return false, fmt.Errorf("request schedule: %w", err)
		}
	}

	err = job.SaveNextInputMapping(inputMapping, resolved)
	if err != nil {
		return false, fmt.Errorf("save next input mapping: %w", err)
	}

	err = s.ensurePendingBuildExists(ctx, logger, job, jobInputs)
	if err != nil {
		return false, err
	}

	return s.BuildStarter.TryStartPendingBuildsForJob(logger, job, jobInputs)
}

func (s *Scheduler) ensurePendingBuildExists(
	ctx context.Context,
	logger lager.Logger,
	job db.SchedulerJob,
	jobInputs db.InputConfigs,
) error {
	buildInputs, satisfiableInputs, err := job.GetFullNextBuildInputs()
	if err != nil {
		return fmt.Errorf("get next build inputs: %w", err)
	}

	if !satisfiableInputs {
		logger.Debug("next-build-inputs-not-determined")
		return nil
	}

	inputMapping := map[string]db.BuildInput{}
	for _, input := range buildInputs {
		inputMapping[input.Name] = input
	}

	var hasNewInputs bool
	for _, inputConfig := range jobInputs {
		inputSource, ok := inputMapping[inputConfig.Name]

		//trigger: true, and the version has not been used
		if ok && inputSource.FirstOccurrence {
			hasNewInputs = true
			if inputConfig.Trigger {
				version, _ := json.Marshal(inputSource.Version)
				spanCtx, _ := tracing.StartSpanLinkedToFollowing(
					ctx,
					inputSource,
					"job.EnsurePendingBuildExists",
					tracing.Attrs{
						"team":     job.TeamName(),
						"pipeline": job.PipelineName(),
						"job":      job.Name(),
						"input":    inputSource.Name,
						"version":  string(version),
					},
				)
				err := job.EnsurePendingBuildExists(spanCtx)
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
