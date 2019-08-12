package emitter

import (
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/metric"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type PrometheusEmitter struct {
	buildDurationsVec *prometheus.HistogramVec
	buildsAborted     prometheus.Counter
	buildsErrored     prometheus.Counter
	buildsFailed      prometheus.Counter
	buildsFinished    prometheus.Counter
	buildsFinishedVec *prometheus.CounterVec
	buildsStarted     prometheus.Counter
	buildsSucceeded   prometheus.Counter

	dbConnections  *prometheus.GaugeVec
	dbQueriesTotal prometheus.Counter

	errorLogs *prometheus.CounterVec

	httpRequestsDuration *prometheus.HistogramVec

	locksHeld *prometheus.GaugeVec

	pipelineScheduled *prometheus.CounterVec

	resourceChecksVec *prometheus.CounterVec

	schedulingFullDuration    *prometheus.CounterVec
	schedulingLoadingDuration *prometheus.CounterVec

	workerContainers  *prometheus.GaugeVec
	workerVolumes     *prometheus.GaugeVec
	workerTasks       *prometheus.GaugeVec
	workersRegistered *prometheus.GaugeVec

	workerLastSeen map[string]time.Time
	mu             sync.Mutex
}

type PrometheusConfig struct {
	BindIP   string `long:"prometheus-bind-ip" description:"IP to listen on to expose Prometheus metrics."`
	BindPort string `long:"prometheus-bind-port" description:"Port to listen on to expose Prometheus metrics."`
}

func init() {
	metric.RegisterEmitter(&PrometheusConfig{})
}

func (config *PrometheusConfig) Description() string { return "Prometheus" }
func (config *PrometheusConfig) IsConfigured() bool {
	return config.BindPort != "" && config.BindIP != ""
}
func (config *PrometheusConfig) bind() string {
	return fmt.Sprintf("%s:%s", config.BindIP, config.BindPort)
}

func (config *PrometheusConfig) NewEmitter() (metric.Emitter, error) {
	// error log metrics
	errorLogs := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "concourse",
			Subsystem: "error",
			Name:      "logs",
			Help:      "Number of error logged",
		}, []string{"message"},
	)
	prometheus.MustRegister(errorLogs)

	// lock metrics
	locksHeld := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "concourse",
		Subsystem: "locks",
		Name:      "held",
		Help:      "Database locks held",
	}, []string{"type"})
	prometheus.MustRegister(locksHeld)

	// build metrics
	buildsStarted := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "concourse",
		Subsystem: "builds",
		Name:      "started_total",
		Help:      "Total number of Concourse builds started.",
	})
	prometheus.MustRegister(buildsStarted)

	buildsFinished := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "concourse",
		Subsystem: "builds",
		Name:      "finished_total",
		Help:      "Total number of Concourse builds finished.",
	})
	prometheus.MustRegister(buildsFinished)

	buildsSucceeded := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "concourse",
		Subsystem: "builds",
		Name:      "succeeded_total",
		Help:      "Total number of Concourse builds succeeded.",
	})
	prometheus.MustRegister(buildsSucceeded)

	buildsErrored := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "concourse",
		Subsystem: "builds",
		Name:      "errored_total",
		Help:      "Total number of Concourse builds errored.",
	})
	prometheus.MustRegister(buildsErrored)

	buildsFailed := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "concourse",
		Subsystem: "builds",
		Name:      "failed_total",
		Help:      "Total number of Concourse builds failed.",
	})
	prometheus.MustRegister(buildsFailed)

	buildsAborted := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "concourse",
		Subsystem: "builds",
		Name:      "aborted_total",
		Help:      "Total number of Concourse builds aborted.",
	})
	prometheus.MustRegister(buildsAborted)

	buildsFinishedVec := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "concourse",
			Subsystem: "builds",
			Name:      "finished",
			Help:      "Count of builds finished across various dimensions.",
		},
		[]string{"team", "pipeline", "job", "status"},
	)
	prometheus.MustRegister(buildsFinishedVec)

	buildDurationsVec := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "concourse",
			Subsystem: "builds",
			Name:      "duration_seconds",
			Help:      "Build time in seconds",
			Buckets:   []float64{1, 60, 180, 300, 600, 900, 1200, 1800, 2700, 3600, 7200, 18000, 36000},
		},
		[]string{"team", "pipeline"},
	)
	prometheus.MustRegister(buildDurationsVec)

	// worker metrics
	workerContainers := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "concourse",
			Subsystem: "workers",
			Name:      "containers",
			Help:      "Number of containers per worker",
		},
		[]string{"worker", "platform", "team", "tags"},
	)
	prometheus.MustRegister(workerContainers)

	workerVolumes := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "concourse",
			Subsystem: "workers",
			Name:      "volumes",
			Help:      "Number of volumes per worker",
		},
		[]string{"worker", "platform", "team", "tags"},
	)
	prometheus.MustRegister(workerVolumes)

	workerTasks := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "concourse",
			Subsystem: "workers",
			Name:      "tasks",
			Help:      "Number of active tasks per worker",
		},
		[]string{"worker", "platform"},
	)
	prometheus.MustRegister(workerTasks)

	workersRegistered := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "concourse",
			Subsystem: "workers",
			Name:      "registered",
			Help:      "Number of workers per state as seen by the database",
		},
		[]string{"state"},
	)
	prometheus.MustRegister(workersRegistered)

	// http metrics
	httpRequestsDuration := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "concourse",
			Subsystem: "http_responses",
			Name:      "duration_seconds",
			Help:      "Response time in seconds",
		},
		[]string{"method", "route", "status"},
	)
	prometheus.MustRegister(httpRequestsDuration)

	// scheduling metrics
	schedulingFullDuration := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "concourse",
			Subsystem: "scheduling",
			Name:      "full_duration_seconds_total",
			Help:      "Total time taken to schedule an entire pipeline",
		},
		[]string{"pipeline"},
	)
	prometheus.MustRegister(schedulingFullDuration)

	schedulingLoadingDuration := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "concourse",
			Subsystem: "scheduling",
			Name:      "loading_duration_seconds_total",
			Help:      "Total time taken to load version information from the database for a pipeline",
		},
		[]string{"pipeline"},
	)
	prometheus.MustRegister(schedulingLoadingDuration)

	pipelineScheduled := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "concourse",
			Subsystem: "scheduling",
			Name:      "total",
			Help:      "Total number of times a pipeline has been scheduled",
		},
		[]string{"pipeline"},
	)
	prometheus.MustRegister(pipelineScheduled)

	dbQueriesTotal := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "concourse",
		Subsystem: "db",
		Name:      "queries_total",
		Help:      "Total number of database Concourse database queries",
	})
	prometheus.MustRegister(dbQueriesTotal)

	dbConnections := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "concourse",
			Subsystem: "db",
			Name:      "connections",
			Help:      "Current number of concourse database connections",
		},
		[]string{"dbname"},
	)
	prometheus.MustRegister(dbConnections)

	resourceChecksVec := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "concourse",
			Subsystem: "resource",
			Name:      "checks_total",
			Help:      "Counts the number of resource checks performed",
		},
		[]string{"team", "pipeline"},
	)
	prometheus.MustRegister(resourceChecksVec)

	listener, err := net.Listen("tcp", config.bind())
	if err != nil {
		return nil, err
	}

	go http.Serve(listener, promhttp.Handler())

	emitter := &PrometheusEmitter{
		buildDurationsVec: buildDurationsVec,
		buildsAborted:     buildsAborted,
		buildsErrored:     buildsErrored,
		buildsFailed:      buildsFailed,
		buildsFinished:    buildsFinished,
		buildsFinishedVec: buildsFinishedVec,
		buildsStarted:     buildsStarted,
		buildsSucceeded:   buildsSucceeded,

		dbConnections:  dbConnections,
		dbQueriesTotal: dbQueriesTotal,

		errorLogs: errorLogs,

		httpRequestsDuration: httpRequestsDuration,

		locksHeld: locksHeld,

		pipelineScheduled: pipelineScheduled,

		resourceChecksVec: resourceChecksVec,

		schedulingFullDuration:    schedulingFullDuration,
		schedulingLoadingDuration: schedulingLoadingDuration,

		workerContainers:  workerContainers,
		workersRegistered: workersRegistered,
		workerLastSeen:    map[string]time.Time{},
		workerVolumes:     workerVolumes,
		workerTasks:       workerTasks,
	}
	go emitter.periodicMetricGC()

	return emitter, nil
}

// Emit processes incoming metrics.
// In order to provide idiomatic Prometheus metrics, we'll have to convert the various
// Event types (differentiated by the less-than-ideal string Name field) into different
// Prometheus metrics.
func (emitter *PrometheusEmitter) Emit(logger lager.Logger, event metric.Event) {

	//update last seen counters, used to gc stale timeseries
	emitter.updateLastSeen(event)

	switch event.Name {
	case "error log":
		emitter.errorLogsMetric(logger, event)
	case "lock held":
		emitter.lock(logger, event)
	case "build started":
		emitter.buildsStarted.Inc()
	case "build finished":
		emitter.buildFinishedMetrics(logger, event)
	case "worker containers":
		emitter.workerContainersMetric(logger, event)
	case "worker volumes":
		emitter.workerVolumesMetric(logger, event)
	case "worker tasks":
		emitter.workerTasksMetric(logger, event)
	case "worker state":
		emitter.workersRegisteredMetric(logger, event)
	case "http response time":
		emitter.httpResponseTimeMetrics(logger, event)
	case "scheduling: full duration (ms)":
		emitter.schedulingMetrics(logger, event)
	case "scheduling: loading versions duration (ms)":
		emitter.schedulingMetrics(logger, event)
	case "scheduling: job duration (ms)":
		emitter.schedulingMetrics(logger, event)
	case "database queries":
		emitter.databaseMetrics(logger, event)
	case "database connections":
		emitter.databaseMetrics(logger, event)
	case "resource checked":
		emitter.resourceMetric(logger, event)
	default:
		// unless we have a specific metric, we do nothing
	}
}

func (emitter *PrometheusEmitter) lock(logger lager.Logger, event metric.Event) {
	lockType, exists := event.Attributes["type"]
	if !exists {
		logger.Error("failed-to-find-type-in-event", fmt.Errorf("expected type to exist in event.Attributes"))
		return
	}

	if event.Value == 1 {
		emitter.locksHeld.WithLabelValues(lockType).Inc()
	} else {
		emitter.locksHeld.WithLabelValues(lockType).Dec()
	}
}

func (emitter *PrometheusEmitter) errorLogsMetric(logger lager.Logger, event metric.Event) {
	message, exists := event.Attributes["message"]
	if !exists {
		logger.Error("failed-to-find-message-in-event",
			fmt.Errorf("expected team_name to exist in event.Attributes"))
		return
	}

	emitter.errorLogs.WithLabelValues(message).Inc()
}

func (emitter *PrometheusEmitter) buildFinishedMetrics(logger lager.Logger, event metric.Event) {
	// concourse_builds_finished_total
	emitter.buildsFinished.Inc()

	// concourse_builds_finished
	team, exists := event.Attributes["team_name"]
	if !exists {
		logger.Error("failed-to-find-team-name-in-event", fmt.Errorf("expected team_name to exist in event.Attributes"))
		return
	}

	pipeline, exists := event.Attributes["pipeline"]
	if !exists {
		logger.Error("failed-to-find-pipeline-in-event", fmt.Errorf("expected pipeline to exist in event.Attributes"))
		return
	}

	job, exists := event.Attributes["job"]
	if !exists {
		logger.Error("failed-to-find-job-in-event", fmt.Errorf("expected job to exist in event.Attributes"))
		return
	}

	buildStatus, exists := event.Attributes["build_status"]
	if !exists {
		logger.Error("failed-to-find-build_status-in-event", fmt.Errorf("expected build_status to exist in event.Attributes"))
		return
	}
	emitter.buildsFinishedVec.WithLabelValues(team, pipeline, job, buildStatus).Inc()

	// concourse_builds_(aborted|succeeded|failed|errored)_total
	switch buildStatus {
	case string(db.BuildStatusAborted):
		// concourse_builds_aborted_total
		emitter.buildsAborted.Inc()
	case string(db.BuildStatusSucceeded):
		// concourse_builds_succeeded_total
		emitter.buildsSucceeded.Inc()
	case string(db.BuildStatusFailed):
		// concourse_builds_failed_total
		emitter.buildsFailed.Inc()
	case string(db.BuildStatusErrored):
		// concourse_builds_errored_total
		emitter.buildsErrored.Inc()
	}

	// concourse_builds_duration_seconds
	duration, ok := event.Value.(float64)
	if !ok {
		logger.Error("build-finished-event-value-type-mismatch", fmt.Errorf("expected event.Value to be a float64"))
		return
	}
	// seconds are the standard prometheus base unit for time
	duration = duration / 1000
	emitter.buildDurationsVec.WithLabelValues(team, pipeline).Observe(duration)
}

func (emitter *PrometheusEmitter) workerContainersMetric(logger lager.Logger, event metric.Event) {
	worker, exists := event.Attributes["worker"]
	if !exists {
		logger.Error("failed-to-find-worker-in-event", fmt.Errorf("expected worker to exist in event.Attributes"))
		return
	}
	platform, exists := event.Attributes["platform"]
	if !exists || platform == "" {
		logger.Error("failed-to-find-platform-in-event", fmt.Errorf("expected platform to exist in event.Attributes"))
		return
	}
	team, exists := event.Attributes["team_name"]
	if !exists {
		logger.Error("failed-to-find-team-name-in-event", fmt.Errorf("expected team_name to exist in event.Attributes"))
		return
	}
	tags, _ := event.Attributes["tags"]

	containers, ok := event.Value.(int)
	if !ok {
		logger.Error("worker-volumes-event-value-type-mismatch", fmt.Errorf("expected event.Value to be an int"))
		return
	}

	emitter.workerContainers.WithLabelValues(worker, platform, team, tags).Set(float64(containers))
}

func (emitter *PrometheusEmitter) workersRegisteredMetric(logger lager.Logger, event metric.Event) {
	state, exists := event.Attributes["state"]
	if !exists {
		logger.Error("failed-to-find-state-in-event", fmt.Errorf("expected state to exist in event.Attributes"))
		return
	}

	count, ok := event.Value.(int)
	if !ok {
		logger.Error("workers-count-event-value-type-mismatch", fmt.Errorf("expected event.Value to be an int"))
		return
	}

	emitter.workersRegistered.WithLabelValues(state).Set(float64(count))
}

func (emitter *PrometheusEmitter) workerVolumesMetric(logger lager.Logger, event metric.Event) {
	worker, exists := event.Attributes["worker"]
	if !exists {
		logger.Error("failed-to-find-worker-in-event", fmt.Errorf("expected worker to exist in event.Attributes"))
		return
	}
	platform, exists := event.Attributes["platform"]
	if !exists || platform == "" {
		logger.Error("failed-to-find-platform-in-event", fmt.Errorf("expected platform to exist in event.Attributes"))
		return
	}
	team, exists := event.Attributes["team_name"]
	if !exists {
		logger.Error("failed-to-find-team-name-in-event", fmt.Errorf("expected team_name to exist in event.Attributes"))
		return
	}
	tags, _ := event.Attributes["tags"]

	volumes, ok := event.Value.(int)
	if !ok {
		logger.Error("worker-volumes-event-value-type-mismatch", fmt.Errorf("expected event.Value to be an int"))
		return
	}

	emitter.workerVolumes.WithLabelValues(worker, platform, team, tags).Set(float64(volumes))
}

func (emitter *PrometheusEmitter) workerTasksMetric(logger lager.Logger, event metric.Event) {
	worker, exists := event.Attributes["worker"]
	if !exists {
		logger.Error("failed-to-find-worker-in-event", fmt.Errorf("expected worker to exist in event.Attributes"))
		return
	}
	platform, exists := event.Attributes["platform"]
	if !exists || platform == "" {
		logger.Error("failed-to-find-platform-in-event", fmt.Errorf("expected platform to exist in event.Attributes"))
		return
	}

	tasks, ok := event.Value.(int)
	if !ok {
		logger.Error("worker-tasks-event-value-type-mismatch", fmt.Errorf("expected event.Value to be an int"))
		return
	}

	emitter.workerTasks.WithLabelValues(worker, platform).Set(float64(tasks))
}

func (emitter *PrometheusEmitter) httpResponseTimeMetrics(logger lager.Logger, event metric.Event) {
	route, exists := event.Attributes["route"]
	if !exists {
		logger.Error("failed-to-find-route-in-event", fmt.Errorf("expected method to exist in event.Attributes"))
		return
	}

	method, exists := event.Attributes["method"]
	if !exists {
		logger.Error("failed-to-find-method-in-event", fmt.Errorf("expected method to exist in event.Attributes"))
		return
	}

	status, exists := event.Attributes["status"]
	if !exists {
		logger.Error("failed-to-find-status-in-event", fmt.Errorf("expected status to exist in event.Attributes"))
		return
	}

	responseTime, ok := event.Value.(float64)
	if !ok {
		logger.Error("http-response-time-event-value-type-mismatch", fmt.Errorf("expected event.Value to be a float64"))
		return
	}

	emitter.httpRequestsDuration.WithLabelValues(method, route, status).Observe(responseTime / 1000)
}

func (emitter *PrometheusEmitter) schedulingMetrics(logger lager.Logger, event metric.Event) {
	pipeline, exists := event.Attributes["pipeline"]
	if !exists {
		logger.Error("failed-to-find-pipeline-in-event", fmt.Errorf("expected pipeline to exist in event.Attributes"))
		return
	}

	duration, ok := event.Value.(float64)
	if !ok {
		logger.Error("scheduling-full-duration-value-type-mismatch", fmt.Errorf("expected event.Value to be a float64"))
		return
	}

	switch event.Name {
	case "scheduling: full duration (ms)":
		// concourse_scheduling_full_duration_seconds_total
		emitter.schedulingFullDuration.WithLabelValues(pipeline).Add(duration / 1000)
		// concourse_scheduling_total
		emitter.pipelineScheduled.WithLabelValues(pipeline).Inc()
	case "scheduling: loading versions duration (ms)":
		// concourse_scheduling_loading_duration_seconds_total
		emitter.schedulingLoadingDuration.WithLabelValues(pipeline).Add(duration / 1000)
	default:
	}
}

func (emitter *PrometheusEmitter) databaseMetrics(logger lager.Logger, event metric.Event) {
	value, ok := event.Value.(int)
	if !ok {
		logger.Error("db-value-type-mismatch", fmt.Errorf("expected event.Value to be a int"))
		return
	}
	switch event.Name {
	case "database queries":
		emitter.dbQueriesTotal.Add(float64(value))
	case "database connections":
		connectionName, exists := event.Attributes["ConnectionName"]
		if !exists {
			logger.Error("failed-to-connection-name-in-event", fmt.Errorf("expected ConnectionName to exist in event.Attributes"))
			return
		}
		emitter.dbConnections.WithLabelValues(connectionName).Set(float64(value))
	default:
	}

}

func (emitter *PrometheusEmitter) resourceMetric(logger lager.Logger, event metric.Event) {
	pipeline, exists := event.Attributes["pipeline"]
	if !exists {
		logger.Error("failed-to-find-pipeline-in-event", fmt.Errorf("expected pipeline to exist in event.Attributes"))
		return
	}
	team, exists := event.Attributes["team_name"]
	if !exists {
		logger.Error("failed-to-find-team-name-in-event", fmt.Errorf("expected team_name to exist in event.Attributes"))
		return
	}

	emitter.resourceChecksVec.WithLabelValues(team, pipeline).Inc()
}

// updateLastSeen tracks for each worker when it last received a metric event.
func (emitter *PrometheusEmitter) updateLastSeen(event metric.Event) {
	emitter.mu.Lock()
	defer emitter.mu.Unlock()
	if worker, exists := event.Attributes["worker"]; exists {
		emitter.workerLastSeen[worker] = time.Now()
	}
}

//periodically remove stale metrics for workers
func (emitter *PrometheusEmitter) periodicMetricGC() {
	for {
		emitter.mu.Lock()
		now := time.Now()
		for worker, lastSeen := range emitter.workerLastSeen {
			if now.Sub(lastSeen) > 5*time.Minute {
				// This is a little stupid but we don't know the platform here,
				// but DeleteLabelValues requires an exact match on the label set.
				// As a workaround we try all known values for  the "platform" label
				for _, platform := range []string{"linux", "windows", "darwin"} {
					emitter.workerContainers.DeleteLabelValues(worker, platform)
					emitter.workerVolumes.DeleteLabelValues(worker, platform)
					emitter.workerTasks.DeleteLabelValues(worker, platform)
				}
				delete(emitter.workerLastSeen, worker)
			}
		}
		emitter.mu.Unlock()
		time.Sleep(60 * time.Second)
	}
}
