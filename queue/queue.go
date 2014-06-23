package queue

import (
	"time"

	"github.com/concourse/atc/builder"
	"github.com/concourse/atc/builds"
	"github.com/concourse/atc/config"
	"github.com/pivotal-golang/lager"
)

type Queuer interface {
	Trigger(config.Job) (builds.Build, error)
	Enqueue(config.Job, config.Resource, builds.Version) (builds.Build, error)
}

type Queue struct {
	logger lager.Logger

	gracePeriod time.Duration

	builder builder.Builder
}

func NewQueue(logger lager.Logger, gracePeriod time.Duration, builder builder.Builder) *Queue {
	return &Queue{
		logger: logger,

		gracePeriod: gracePeriod,

		builder: builder,
	}
}

func (q *Queue) Trigger(job config.Job) (builds.Build, error) {
	q.logger.Info("queue", "triggering", "", lager.Data{
		"job": job.Name,
	})

	build, err := q.builder.Create(job)
	if err != nil {
		q.logger.Error("queue", "trigger-failed", "", err, lager.Data{
			"job": job.Name,
		})

		return builds.Build{}, err
	}

	q.tryToStart(job, build, nil)

	return build, nil
}

func (q *Queue) Enqueue(job config.Job, resource config.Resource, version builds.Version) (builds.Build, error) {
	q.logger.Info("queue", "attempting", "", lager.Data{
		"job":      job.Name,
		"resource": resource.Name,
		"version":  version,
	})

	build, err := q.builder.Attempt(job, resource, version)
	if err != nil {
		q.logger.Error("queue", "attempt-failed", "", err, lager.Data{
			"job":      job.Name,
			"resource": resource.Name,
			"version":  version,
		})

		return builds.Build{}, err
	}

	q.tryToStart(job, build, map[string]builds.Version{
		resource.Name: version,
	})

	return build, nil
}

func (q *Queue) tryToStart(job config.Job, build builds.Build, versions map[string]builds.Version) {
	q.logger.Info("queue", "starting", "", lager.Data{
		"job":      job.Name,
		"versions": versions,
	})

	build, err := q.builder.Start(job, build, versions)
	if err != nil {
		q.logger.Error("queue", "start-failed", "", err, lager.Data{
			"job":      job.Name,
			"versions": versions,
		})

		return
	}

	if build.Status == builds.StatusPending {
		time.AfterFunc(q.gracePeriod, func() {
			q.tryToStart(job, build, versions)
		})
	}
}
