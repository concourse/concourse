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
	log := q.logger.Session("trigger", lager.Data{
		"job": job.Name,
	})

	log.Debug("create")

	build, err := q.builder.Create(job)
	if err != nil {
		log.Error("create-failed", err)
		return builds.Build{}, err
	}

	q.tryToStart(job, build, nil)

	return build, nil
}

func (q *Queue) Enqueue(job config.Job, resource config.Resource, version builds.Version) (builds.Build, error) {
	log := q.logger.Session("enqueue", lager.Data{
		"job":      job.Name,
		"resource": resource.Name,
		"version":  version,
	})

	build, err := q.builder.Attempt(job, resource, version)
	if err != nil {
		log.Error("attempt-failed", err)
		return builds.Build{}, err
	}

	q.tryToStart(job, build, map[string]builds.Version{
		resource.Name: version,
	})

	return build, nil
}

func (q *Queue) tryToStart(job config.Job, build builds.Build, versions map[string]builds.Version) {
	log := q.logger.Session("try", lager.Data{
		"job":      job.Name,
		"versions": versions,
	})

	log.Debug("start")

	build, err := q.builder.Start(job, build, versions)
	if err != nil {
		log.Error("failed-to-start", err)
		return
	}

	if build.Status == builds.StatusPending {
		log.Info("still-pending")

		time.AfterFunc(q.gracePeriod, func() {
			q.tryToStart(job, build, versions)
		})
	} else {
		log.Info("started")
	}
}
