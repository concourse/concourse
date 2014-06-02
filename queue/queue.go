package queue

import (
	"log"
	"os"
	"sync"
	"time"

	"github.com/winston-ci/winston/builder"
	"github.com/winston-ci/winston/builds"
	"github.com/winston-ci/winston/config"
)

type Queuer interface {
	Trigger(config.Job) (<-chan builds.Build, <-chan error)
	Enqueue(config.Job, config.Resource, builds.Version) (<-chan builds.Build, <-chan error)
}

type Queue struct {
	gracePeriod time.Duration

	queue   chan queuedBuild
	preempt chan preemptedBuild
	trigger chan config.Job

	builder builder.Builder

	pending map[string]pendingBuild

	inFlight *sync.WaitGroup
}

type queuedBuild struct {
	Job      config.Job
	Resource config.Resource
	Version  builds.Version

	StartedBuild chan<- builds.Build
	QueuedBuild  chan<- bool
	BuildErr     chan<- error
}

type preemptedBuild struct {
	Job config.Job

	StartedBuild chan<- builds.Build
	QueuedBuild  chan<- bool
	BuildErr     chan<- error
}

type pendingBuild struct {
	Versions map[string]builds.Version
	Delay    *time.Timer

	StartedBuild []chan<- builds.Build
	BuildErr     []chan<- error
}

func NewQueue(gracePeriod time.Duration, builder builder.Builder) *Queue {
	return &Queue{
		gracePeriod: gracePeriod,

		queue:   make(chan queuedBuild),
		preempt: make(chan preemptedBuild),
		trigger: make(chan config.Job),

		builder: builder,

		pending: make(map[string]pendingBuild),

		inFlight: new(sync.WaitGroup),
	}
}

func (q *Queue) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	go q.handle()

	close(ready)

	<-signals

	q.inFlight.Wait()

	return nil
}

func (q *Queue) Trigger(job config.Job) (<-chan builds.Build, <-chan error) {
	startedBuild := make(chan builds.Build, 1)
	buildErr := make(chan error, 1)
	queued := make(chan bool)

	q.preempt <- preemptedBuild{
		Job: job,

		StartedBuild: startedBuild,
		QueuedBuild:  queued,
		BuildErr:     buildErr,
	}

	queuedNew := <-queued
	if queuedNew {
		q.inFlight.Add(1)
	}

	return startedBuild, buildErr
}

func (q *Queue) Enqueue(job config.Job, resource config.Resource, version builds.Version) (<-chan builds.Build, <-chan error) {
	startedBuild := make(chan builds.Build, 1)
	buildErr := make(chan error, 1)
	queued := make(chan bool)

	q.queue <- queuedBuild{
		Job:      job,
		Resource: resource,
		Version:  version,

		StartedBuild: startedBuild,
		QueuedBuild:  queued,
		BuildErr:     buildErr,
	}

	queuedNew := <-queued
	if queuedNew {
		q.inFlight.Add(1)
	}

	return startedBuild, buildErr
}

func (q *Queue) handle() {
	for {
		select {
		case queued := <-q.queue:
			log.Println("queue enqueuing", queued.Job.Name, queued.Resource.Name, queued.Version)
			q.enqueueBuild(queued)

		case preempted := <-q.preempt:
			log.Println("queue preempting", preempted.Job.Name)
			q.preemptBuild(preempted)

		case job := <-q.trigger:
			log.Println("queue triggering", job.Name)
			q.triggerBuild(job)
		}
	}
}

func (q *Queue) enqueueBuild(queued queuedBuild) {
	pending, found := q.pending[queued.Job.Name]

	queued.QueuedBuild <- !found

	if found {
		pending.Versions[queued.Resource.Name] = queued.Version
		pending.StartedBuild = append(pending.StartedBuild, queued.StartedBuild)
		pending.BuildErr = append(pending.BuildErr, queued.BuildErr)

		q.pending[queued.Job.Name] = pending

	} else {
		q.pending[queued.Job.Name] = pendingBuild{
			Versions: map[string]builds.Version{
				queued.Resource.Name: queued.Version,
			},
			Delay: time.AfterFunc(q.gracePeriod, func() {
				q.trigger <- queued.Job
			}),
			StartedBuild: []chan<- builds.Build{queued.StartedBuild},
			BuildErr:     []chan<- error{queued.BuildErr},
		}
	}
}

func (q *Queue) preemptBuild(preempted preemptedBuild) {
	pending, found := q.pending[preempted.Job.Name]

	preempted.QueuedBuild <- !found

	if found {
		pending.StartedBuild = append(pending.StartedBuild, preempted.StartedBuild)
		pending.BuildErr = append(pending.BuildErr, preempted.BuildErr)

		q.pending[preempted.Job.Name] = pending

		pending.Delay.Reset(0)
	} else {
		q.pending[preempted.Job.Name] = pendingBuild{
			Delay: time.AfterFunc(0, func() {
				q.trigger <- preempted.Job
			}),
			StartedBuild: []chan<- builds.Build{preempted.StartedBuild},
			BuildErr:     []chan<- error{preempted.BuildErr},
		}
	}
}

func (q *Queue) triggerBuild(job config.Job) {
	pending, found := q.pending[job.Name]
	if !found {
		return
	}

	delete(q.pending, job.Name)

	build, err := q.builder.Create(job)

	if err != nil {
		for _, errs := range pending.BuildErr {
			errs <- err
		}
	} else {
		for _, builds := range pending.StartedBuild {
			builds <- build
		}
	}

	q.tryToStart(job, build, pending.Versions)

	q.inFlight.Done()
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
