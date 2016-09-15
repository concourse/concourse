package metric

import (
	"strconv"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/bigdatadev/goryman"

	"github.com/concourse/atc/db"
)

var TrackedContainers = &Gauge{}
var TrackedVolumes = &Gauge{}
var DatabaseQueries = Meter(0)
var DatabaseConnections = &Gauge{}

type SchedulingFullDuration struct {
	PipelineName string
	Duration     time.Duration
}

func (event SchedulingFullDuration) Emit(logger lager.Logger) {
	state := "ok"

	if event.Duration > time.Second {
		state = "warning"
	}

	if event.Duration > 5*time.Second {
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
	}

	if event.Duration > 5*time.Second {
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
	}

	if event.Duration > 5*time.Second {
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
	WorkerName string
	Containers int
}

func (event WorkerContainers) Emit(logger lager.Logger) {
	emit(
		logger.Session("worker-containers", lager.Data{
			"worker":     event.WorkerName,
			"containers": event.Containers,
		}),
		goryman.Event{
			Service: "worker containers",
			Metric:  event.Containers,
			State:   "ok",
			Attributes: map[string]string{
				"worker": event.WorkerName,
			},
		},
	)
}

type BuildStarted struct {
	PipelineName string
	JobName      string
	BuildName    string
	BuildID      int
}

func (event BuildStarted) Emit(logger lager.Logger) {
	emit(
		logger.Session("build-started", lager.Data{
			"pipeline":   event.PipelineName,
			"job":        event.JobName,
			"build-name": event.BuildName,
			"build-id":   event.BuildID,
		}),
		goryman.Event{
			Service: "build started",
			Metric:  event.BuildID,
			State:   "ok",
			Attributes: map[string]string{
				"pipeline":   event.PipelineName,
				"job":        event.JobName,
				"build_name": event.BuildName,
				"build_id":   strconv.Itoa(event.BuildID),
			},
		},
	)
}

type BuildFinished struct {
	PipelineName  string
	JobName       string
	BuildName     string
	BuildID       int
	BuildStatus   db.Status
	BuildDuration time.Duration
}

func (event BuildFinished) Emit(logger lager.Logger) {
	emit(
		logger.Session("build-finished", lager.Data{
			"pipeline":     event.PipelineName,
			"job":          event.JobName,
			"build-name":   event.BuildName,
			"build-id":     event.BuildID,
			"build-status": event.BuildStatus,
		}),
		goryman.Event{
			Service: "build finished",
			Metric:  ms(event.BuildDuration),
			State:   "ok",
			Attributes: map[string]string{
				"pipeline":     event.PipelineName,
				"job":          event.JobName,
				"build_name":   event.BuildName,
				"build_id":     strconv.Itoa(event.BuildID),
				"build_status": string(event.BuildStatus),
			},
		},
	)
}

func ms(duration time.Duration) float64 {
	return float64(duration) / 1000000
}

type HTTPReponseTime struct {
	Route    string
	Path     string
	Duration time.Duration
}

func (event HTTPReponseTime) Emit(logger lager.Logger) {
	state := "ok"

	if event.Duration > 100*time.Millisecond {
		state = "warning"
	}

	if event.Duration > 1*time.Second {
		state = "critical"
	}

	emit(
		logger.Session("http-response-time", lager.Data{
			"route":    event.Route,
			"path":     event.Path,
			"duration": event.Duration.String(),
		}),
		goryman.Event{
			Service: "http response time",
			Metric:  ms(event.Duration),
			State:   state,
			Attributes: map[string]string{
				"route": event.Route,
				"path":  event.Path,
			},
		},
	)
}
