package metric

import (
	"time"

	"github.com/bigdatadev/goryman"
	"github.com/pivotal-golang/lager"
)

type Event interface {
	Emit(lager.Logger)
}

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

	emit(eventEmission{
		event: goryman.Event{
			Service: "scheduling: full duration (ms)",
			Metric:  ms(event.Duration),
			State:   state,
			Tags:    []string{"pipeline:" + event.PipelineName},
		},

		logger: logger.Session("full-scheduling-duration", lager.Data{
			"pipeline": event.PipelineName,
			"duration": event.Duration.String(),
		}),
	})
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

	emit(eventEmission{
		event: goryman.Event{
			Service: "scheduling: loading versions duration (ms)",
			Metric:  ms(event.Duration),
			State:   state,
			Tags:    []string{"pipeline:" + event.PipelineName},
		},

		logger: logger.Session("loading-versions-duration", lager.Data{
			"pipeline": event.PipelineName,
			"duration": event.Duration.String(),
		}),
	})
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

	emit(eventEmission{
		event: goryman.Event{
			Service: "scheduling: job duration (ms)",
			Metric:  ms(event.Duration),
			State:   state,
			Tags:    []string{"pipeline:" + event.PipelineName, "job:" + event.JobName},
		},

		logger: logger.Session("job-scheduling-duration", lager.Data{
			"pipeline": event.PipelineName,
			"job":      event.JobName,
			"duration": event.Duration.String(),
		}),
	})
}

type WorkerContainers struct {
	WorkerAddr string
	Containers int
}

func (event WorkerContainers) Emit(logger lager.Logger) {
	emit(eventEmission{
		event: goryman.Event{
			Service: "scheduling: job duration (ms)",
			Metric:  event.Containers,
			State:   "ok",
			Tags:    []string{"worker:" + event.WorkerAddr},
		},

		logger: logger.Session("worker-containers", lager.Data{
			"worker":     event.WorkerAddr,
			"containers": event.Containers,
		}),
	})
}

func ms(duration time.Duration) float64 {
	return float64(duration) / 1000000
}
