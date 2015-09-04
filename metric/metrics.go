package metric

import (
	"time"

	"github.com/bigdatadev/goryman"
	"github.com/pivotal-golang/lager"
)

type Event interface {
	Emit(lager.Logger)
}

var TrackedContainers = &Counter{}

type SchedulingFullDuration struct {
	PipelineName string
	Duration     time.Duration
}

func (event SchedulingFullDuration) Emit(logger lager.Logger) {
	state := "ok"
	if event.Duration > time.Second {
		state = "warning"
	} else if event.Duration > 5*time.Second {
		state = "critical"
	}

	emit(
		logger.Session("full-scheduling-duration", lager.Data{
			"pipeline": event.PipelineName,
			"duration": event.Duration.String(),
		}),

		goryman.Event{
			Service: "scheduling: full duration (ms)",
			Metric:  ms(event.Duration),
			State:   state,
			Attributes: map[string]string{
				"pipeline": event.PipelineName,
			},
		},
	)
}

type SchedulingLoadVersionsDuration struct {
	PipelineName string
	Duration     time.Duration
}

func (event SchedulingLoadVersionsDuration) Emit(logger lager.Logger) {
	state := "ok"
	if event.Duration > time.Second {
		state = "warning"
	} else if event.Duration > 5*time.Second {
		state = "critical"
	}

	emit(
		logger.Session("loading-versions-duration", lager.Data{
			"pipeline": event.PipelineName,
			"duration": event.Duration.String(),
		}),
		goryman.Event{
			Service: "scheduling: loading versions duration (ms)",
			Metric:  ms(event.Duration),
			State:   state,
			Attributes: map[string]string{
				"pipeline": event.PipelineName,
			},
		},
	)
}

type SchedulingJobDuration struct {
	PipelineName string
	JobName      string
	Duration     time.Duration
}

func (event SchedulingJobDuration) Emit(logger lager.Logger) {
	state := "ok"
	if event.Duration > time.Second {
		state = "warning"
	} else if event.Duration > 5*time.Second {
		state = "critical"
	}

	emit(
		logger.Session("job-scheduling-duration", lager.Data{
			"pipeline": event.PipelineName,
			"job":      event.JobName,
			"duration": event.Duration.String(),
		}),
		goryman.Event{
			Service: "scheduling: job duration (ms)",
			Metric:  ms(event.Duration),
			State:   state,
			Attributes: map[string]string{
				"pipeline": event.PipelineName,
				"job":      event.JobName,
			},
		},
	)
}

type WorkerContainers struct {
	WorkerAddr string
	Containers int
}

func (event WorkerContainers) Emit(logger lager.Logger) {
	emit(
		logger.Session("worker-containers", lager.Data{
			"worker":     event.WorkerAddr,
			"containers": event.Containers,
		}),
		goryman.Event{
			Service: "worker containers",
			Metric:  event.Containers,
			State:   "ok",
			Attributes: map[string]string{
				"worker": event.WorkerAddr,
			},
		},
	)
}

func ms(duration time.Duration) float64 {
	return float64(duration) / 1000000
}
