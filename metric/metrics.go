package metric

import (
	"fmt"
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

const msConversion = 1000000

func (event SchedulingFullDuration) Emit(logger lager.Logger) {
	state := "ok"
	if event.Duration > time.Second {
		state = "warning"
	} else if event.Duration > 5*time.Second {
		state = "critical"
	}

	emit(eventEmission{
		event: goryman.Event{
			Service: fmt.Sprintf("scheduling: full duration (ms, pipeline: %s)", event.PipelineName),
			Metric:  ms(event.Duration),
			State:   state,
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
			Service: fmt.Sprintf("scheduling: loading versions duration (ms, pipeline: %s)", event.PipelineName),
			Metric:  ms(event.Duration),
			State:   state,
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
			Service: fmt.Sprintf("scheduling: job duration (ms, pipeline: %s, job: %s)", event.PipelineName, event.JobName),
			Metric:  ms(event.Duration),
			State:   state,
		},

		logger: logger.Session("job-scheduling-duration", lager.Data{
			"pipeline": event.PipelineName,
			"job":      event.JobName,
			"duration": event.Duration.String(),
		}),
	})
}

func ms(duration time.Duration) float64 {
	return float64(duration) / 1000000
}
