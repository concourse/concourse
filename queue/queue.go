package queue

import (
	"log"
	"time"

	"github.com/concourse/atc/builder"
	"github.com/concourse/atc/builds"
	"github.com/concourse/atc/config"
)

type Queuer interface {
	Trigger(config.Job) (builds.Build, error)
	Enqueue(config.Job, config.Resource, builds.Version) (builds.Build, error)
}

type Queue struct {
	gracePeriod time.Duration

	builder builder.Builder
}

func NewQueue(gracePeriod time.Duration, builder builder.Builder) *Queue {
	return &Queue{
		gracePeriod: gracePeriod,

		builder: builder,
	}
}

func (q *Queue) Trigger(job config.Job) (builds.Build, error) {
	build, err := q.builder.Create(job)
	if err != nil {
		log.Println("queue errored creating build:", err)
		return builds.Build{}, err
	}

	q.tryToStart(job, build, nil)

	return build, nil
}

func (q *Queue) Enqueue(job config.Job, resource config.Resource, version builds.Version) (builds.Build, error) {
	build, err := q.builder.Attempt(job, resource, version)
	if err != nil {
		log.Println("queue errored attempting build:", err)
		return builds.Build{}, err
	}

	q.tryToStart(job, build, map[string]builds.Version{
		resource.Name: version,
	})

	return build, nil
}

func (q *Queue) tryToStart(job config.Job, build builds.Build, versions map[string]builds.Version) {
	build, err := q.builder.Start(job, build, versions)
	if err != nil {
		log.Println("queue errored starting build:", err)
		return
	}

	if build.Status == builds.StatusPending {
		time.AfterFunc(q.gracePeriod, func() {
			q.tryToStart(job, build, versions)
		})
	}
}
