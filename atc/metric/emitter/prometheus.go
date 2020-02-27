package emitter

import (
	"fmt"
	"net"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/metric"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const concourseNameSpace = "concourse"
const (
	errorSubSystem         = "error"
	locksSubSystem         = "locks"
	jobsSubSystem          = "jobs"
	buildsSubSystem        = "builds"
	workersSubSystem       = "workers"
	httpResponsesSubSystem = "http_responses"
	dbSubSystem            = "db"
	resourceSubSystem      = "resource"
	lidarSubSystem         = "lidar"
)

type PrometheusEmitter struct {
	jobsScheduled  prometheus.Counter
	jobsScheduling prometheus.Gauge

	buildsStarted prometheus.Counter
	buildsRunning prometheus.Gauge

	buildDurationsVec *prometheus.HistogramVec
	buildsAborted     prometheus.Counter
	buildsErrored     prometheus.Counter
	buildsFailed      prometheus.Counter
	buildsFinished    prometheus.Counter
	buildsFinishedVec *prometheus.CounterVec
	buildsSucceeded   prometheus.Counter

	dbConnections  *prometheus.GaugeVec
	dbQueriesTotal prometheus.Counter

	errorLogs *prometheus.CounterVec

	httpRequestsDuration *prometheus.HistogramVec

	locksHeld *prometheus.GaugeVec

	resourceChecksVec *prometheus.CounterVec

	checksVec       *prometheus.HistogramVec
	checkEnqueueVec *prometheus.CounterVec
	checkQueueSize  prometheus.Gauge

	workerContainers        *prometheus.GaugeVec
	workerUnknownContainers *prometheus.GaugeVec
	workerVolumes           *prometheus.GaugeVec
	workerUnknownVolumes    *prometheus.GaugeVec
	workerTasks             *prometheus.GaugeVec
	workersRegistered       *prometheus.GaugeVec

	workerContainersLabels map[string]map[string]prometheus.Labels
	workerVolumesLabels    map[string]map[string]prometheus.Labels
	workerTasksLabels      map[string]map[string]prometheus.Labels
	workerLastSeen         map[string]time.Time
	mu                     sync.Mutex
}

type PrometheusConfig struct {
	BindIP   string `long:"prometheus-bind-ip" description:"IP to listen on to expose Prometheus metrics."`
	BindPort string `long:"prometheus-bind-port" description:"Port to listen on to expose Prometheus metrics."`
}

// The most natural data type to hold the labels is a set because each worker can have multiple but
// unique sets of labels. A set in Go is represented by a map[T]struct{}. Unfortunately, we cannot
// put prometheus.Labels inside a map[prometheus.Labels]struct{} because prometheus.Labels are not
// hashable. To work around this, we compute a string from the labels and use this as the keys of
// the map.
func serializeLabels(labels *prometheus.Labels) string {
	var (
		key   string
		names []string
	)
	for _, v := range *labels {
		names = append(names, v)
	}
	sort.Strings(names)
	key = strings.Join(names, "_")

	return key
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
			Namespace: concourseNameSpace,
			Subsystem: errorSubSystem,
			Name:      "logs",
			Help:      "Number of error logged",
		}, []string{"message"},
	)
	prometheus.MustRegister(errorLogs)

	// lock metrics
	locksHeld := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: concourseNameSpace,
		Subsystem: locksSubSystem,
		Name:      "held",
		Help:      "Database locks held",
	}, []string{"type"})
	prometheus.MustRegister(locksHeld)

	// job metrics
	jobsScheduled := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: concourseNameSpace,
		Subsystem: jobsSubSystem,
		Name:      "scheduled_total",
		Help:      "Total number of Concourse jobs scheduled.",
	})
	prometheus.MustRegister(jobsScheduled)

	jobsScheduling := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: concourseNameSpace,
		Subsystem: jobsSubSystem,
		Name:      "scheduling",
		Help:      "Number of Concourse jobs currently being scheduled.",
	})
	prometheus.MustRegister(jobsScheduling)

	// build metrics
	buildsStarted := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: concourseNameSpace,
		Subsystem: buildsSubSystem,
		Name:      "started_total",
		Help:      "Total number of Concourse builds started.",
	})
	prometheus.MustRegister(buildsStarted)

	buildsRunning := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: concourseNameSpace,
		Subsystem: buildsSubSystem,
		Name:      "running",
		Help:      "Number of Concourse builds currently running.",
	})
	prometheus.MustRegister(buildsRunning)

	buildsFinished := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: concourseNameSpace,
		Subsystem: buildsSubSystem,
		Name:      "finished_total",
		Help:      "Total number of Concourse builds finished.",
	})
	prometheus.MustRegister(buildsFinished)

	buildsSucceeded := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: concourseNameSpace,
		Subsystem: buildsSubSystem,
		Name:      "succeeded_total",
		Help:      "Total number of Concourse builds succeeded.",
	})
	prometheus.MustRegister(buildsSucceeded)

	buildsErrored := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: concourseNameSpace,
		Subsystem: buildsSubSystem,
		Name:      "errored_total",
		Help:      "Total number of Concourse builds errored.",
	})
	prometheus.MustRegister(buildsErrored)

	buildsFailed := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: concourseNameSpace,
		Subsystem: buildsSubSystem,
		Name:      "failed_total",
		Help:      "Total number of Concourse builds failed.",
	})
	prometheus.MustRegister(buildsFailed)

	buildsAborted := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: concourseNameSpace,
		Subsystem: buildsSubSystem,
		Name:      "aborted_total",
		Help:      "Total number of Concourse builds aborted.",
	})
	prometheus.MustRegister(buildsAborted)

	buildsFinishedVec := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: concourseNameSpace,
			Subsystem: buildsSubSystem,
			Name:      "finished",
			Help:      "Count of builds finished across various dimensions.",
		},
		[]string{"team", "pipeline", "job", "status"},
	)
	prometheus.MustRegister(buildsFinishedVec)

	buildDurationsVec := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: concourseNameSpace,
			Subsystem: buildsSubSystem,
			Name:      "duration_seconds",
			Help:      "Build time in seconds",
			Buckets:   []float64{1, 60, 180, 300, 600, 900, 1200, 1800, 2700, 3600, 7200, 18000, 36000},
		},
		[]string{"team", "pipeline", "job"},
	)
	prometheus.MustRegister(buildDurationsVec)

	// worker metrics
	workerContainers := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: concourseNameSpace,
			Subsystem: workersSubSystem,
			Name:      "containers",
			Help:      "Number of containers per worker",
		},
		[]string{"worker", "platform", "team", "tags"},
	)
	prometheus.MustRegister(workerContainers)

	workerUnknownContainers := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: concourseNameSpace,
			Subsystem: workersSubSystem,
			Name:      "unknown_containers",
			Help:      "Number of unknown containers found on worker",
		},
		[]string{"worker"},
	)
	prometheus.MustRegister(workerUnknownContainers)

	workerVolumes := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: concourseNameSpace,
			Subsystem: workersSubSystem,
			Name:      "volumes",
			Help:      "Number of volumes per worker",
		},
		[]string{"worker", "platform", "team", "tags"},
	)
	prometheus.MustRegister(workerVolumes)

	workerUnknownVolumes := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: concourseNameSpace,
			Subsystem: workersSubSystem,
			Name:      "unknown_volumes",
			Help:      "Number of unknown volumes found on worker",
		},
		[]string{"worker"},
	)
	prometheus.MustRegister(workerUnknownVolumes)

	workerTasks := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: concourseNameSpace,
			Subsystem: workersSubSystem,
			Name:      "tasks",
			Help:      "Number of active tasks per worker",
		},
		[]string{"worker", "platform"},
	)
	prometheus.MustRegister(workerTasks)

	workersRegistered := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: concourseNameSpace,
			Subsystem: workersSubSystem,
			Name:      "registered",
			Help:      "Number of workers per state as seen by the database",
		},
		[]string{"state"},
	)
	prometheus.MustRegister(workersRegistered)

	// http metrics
	httpRequestsDuration := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: concourseNameSpace,
			Subsystem: httpResponsesSubSystem,
			Name:      "duration_seconds",
			Help:      "Response time in seconds",
		},
		[]string{"method", "route", "status"},
	)
	prometheus.MustRegister(httpRequestsDuration)

	dbQueriesTotal := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: concourseNameSpace,
		Subsystem: dbSubSystem,
		Name:      "queries_total",
		Help:      "Total number of database Concourse database queries",
	})
	prometheus.MustRegister(dbQueriesTotal)

	dbConnections := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: concourseNameSpace,
			Subsystem: dbSubSystem,
			Name:      "connections",
			Help:      "Current number of concourse database connections",
		},
		[]string{"dbname"},
	)
	prometheus.MustRegister(dbConnections)

	resourceChecksVec := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: concourseNameSpace,
			Subsystem: resourceSubSystem,
			Name:      "checks_total",
			Help:      "Counts the number of resource checks performed",
		},
		[]string{"team", "pipeline"},
	)
	prometheus.MustRegister(resourceChecksVec)

	checksVec := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: concourseNameSpace,
			Subsystem: lidarSubSystem,
			Name:      "duration_seconds",
			Help:      "Check time in seconds",
			Buckets:   []float64{0.001, 0.05, 0.1, 0.5, 1, 60, 180, 360, 720, 1440, 2880},
		},
		[]string{"scope_id", "status"},
	)
	prometheus.MustRegister(checksVec)

	checkEnqueueVec := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: concourseNameSpace,
			Subsystem: lidarSubSystem,
			Name:      "check_enqueue",
			Help:      "Counts the number of checks enqueued",
		},
		[]string{"scope_id", "name"},
	)
	prometheus.MustRegister(checkEnqueueVec)

	checkQueueSize := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: concourseNameSpace,
			Subsystem: lidarSubSystem,
			Name:      "check_queue_size",
			Help:      "Records the size of check queue",
		},
	)
	prometheus.MustRegister(checkQueueSize)

	listener, err := net.Listen("tcp", config.bind())
	if err != nil {
		return nil, err
	}

	go http.Serve(listener, promhttp.Handler())

	emitter := &PrometheusEmitter{
		jobsScheduled:  jobsScheduled,
		jobsScheduling: jobsScheduling,

		buildsStarted: buildsStarted,
		buildsRunning: buildsRunning,

		buildDurationsVec: buildDurationsVec,
		buildsAborted:     buildsAborted,
		buildsErrored:     buildsErrored,
		buildsFailed:      buildsFailed,
		buildsFinished:    buildsFinished,
		buildsFinishedVec: buildsFinishedVec,
		buildsSucceeded:   buildsSucceeded,

		dbConnections:  dbConnections,
		dbQueriesTotal: dbQueriesTotal,

		errorLogs: errorLogs,

		httpRequestsDuration: httpRequestsDuration,

		locksHeld: locksHeld,

		resourceChecksVec: resourceChecksVec,

		checksVec:       checksVec,
		checkEnqueueVec: checkEnqueueVec,
		checkQueueSize:  checkQueueSize,

		workerContainers:        workerContainers,
		workersRegistered:       workersRegistered,
		workerContainersLabels:  map[string]map[string]prometheus.Labels{},
		workerVolumesLabels:     map[string]map[string]prometheus.Labels{},
		workerTasksLabels:       map[string]map[string]prometheus.Labels{},
		workerLastSeen:          map[string]time.Time{},
		workerVolumes:           workerVolumes,
		workerTasks:             workerTasks,
		workerUnknownContainers: workerUnknownContainers,
		workerUnknownVolumes:    workerUnknownVolumes,
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
	case "jobs scheduled":
		emitter.jobsScheduled.Add(event.Value)
	case "jobs scheduling":
		emitter.jobsScheduling.Set(event.Value)
	case "builds started":
		emitter.buildsStarted.Add(event.Value)
	case "builds running":
		emitter.buildsRunning.Set(event.Value)
	case "build finished":
		emitter.buildFinishedMetrics(logger, event)
	case "worker containers":
		emitter.workerContainersMetric(logger, event)
	case "worker volumes":
		emitter.workerVolumesMetric(logger, event)
	case "worker unknown containers":
		emitter.workerUnknownContainersMetric(logger, event)
	case "worker unknown volumes":
		emitter.workerUnknownVolumesMetric(logger, event)
	case "worker tasks":
		emitter.workerTasksMetric(logger, event)
	case "worker state":
		emitter.workersRegisteredMetric(logger, event)
	case "http response time":
		emitter.httpResponseTimeMetrics(logger, event)
	case "database queries":
		emitter.databaseMetrics(logger, event)
	case "database connections":
		emitter.databaseMetrics(logger, event)
	case "resource checked":
		emitter.resourceMetric(logger, event)
	case "check enqueued":
		emitter.checkEnqueueMetric(logger, event)
	case "check queue size":
		emitter.checkQueueSizeMetric(logger, event)
	case "check started":
		emitter.checkMetric(logger, event)
	case "check finished":
		emitter.checkMetric(logger, event)
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

	// seconds are the standard prometheus base unit for time
	duration := event.Value / 1000
	emitter.buildDurationsVec.WithLabelValues(team, pipeline, job).Observe(duration)
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

	labels := prometheus.Labels{
		"worker":   worker,
		"platform": platform,
		"team":     team,
		"tags":     tags,
	}
	key := serializeLabels(&labels)
	if emitter.workerContainersLabels[worker] == nil {
		emitter.workerContainersLabels[worker] = make(map[string]prometheus.Labels)
	}
	emitter.workerContainersLabels[worker][key] = labels
	emitter.workerContainers.With(emitter.workerContainersLabels[worker][key]).Set(event.Value)
}

func (emitter *PrometheusEmitter) workersRegisteredMetric(logger lager.Logger, event metric.Event) {
	state, exists := event.Attributes["state"]
	if !exists {
		logger.Error("failed-to-find-state-in-event", fmt.Errorf("expected state to exist in event.Attributes"))
		return
	}

	emitter.workersRegistered.WithLabelValues(state).Set(event.Value)
}

func (emitter *PrometheusEmitter) workerUnknownContainersMetric(logger lager.Logger, event metric.Event) {
	worker, exists := event.Attributes["worker"]
	if !exists {
		logger.Error("failed-to-find-worker-in-event", fmt.Errorf("expected worker to exist in event.Attributes"))
		return
	}

	labels := prometheus.Labels{
		"worker": worker,
	}

	key := serializeLabels(&labels)
	if emitter.workerContainersLabels[worker] == nil {
		emitter.workerContainersLabels[worker] = make(map[string]prometheus.Labels)
	}
	emitter.workerContainersLabels[worker][key] = labels
	emitter.workerUnknownContainers.With(emitter.workerContainersLabels[worker][key]).Set(event.Value)
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

	labels := prometheus.Labels{
		"worker":   worker,
		"platform": platform,
		"team":     team,
		"tags":     tags,
	}
	key := serializeLabels(&labels)
	if emitter.workerVolumesLabels[worker] == nil {
		emitter.workerVolumesLabels[worker] = make(map[string]prometheus.Labels)
	}
	emitter.workerVolumesLabels[worker][key] = labels
	emitter.workerVolumes.With(emitter.workerVolumesLabels[worker][key]).Set(event.Value)
}

func (emitter *PrometheusEmitter) workerUnknownVolumesMetric(logger lager.Logger, event metric.Event) {
	worker, exists := event.Attributes["worker"]
	if !exists {
		logger.Error("failed-to-find-worker-in-event", fmt.Errorf("expected worker to exist in event.Attributes"))
		return
	}

	labels := prometheus.Labels{
		"worker": worker,
	}

	key := serializeLabels(&labels)
	if emitter.workerVolumesLabels[worker] == nil {
		emitter.workerVolumesLabels[worker] = make(map[string]prometheus.Labels)
	}
	emitter.workerVolumesLabels[worker][key] = labels
	emitter.workerUnknownVolumes.With(emitter.workerVolumesLabels[worker][key]).Set(event.Value)
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

	labels := prometheus.Labels{
		"worker":   worker,
		"platform": platform,
	}
	key := serializeLabels(&labels)
	if emitter.workerTasksLabels[worker] == nil {
		emitter.workerTasksLabels[worker] = make(map[string]prometheus.Labels)
	}
	emitter.workerTasksLabels[worker][key] = labels
	emitter.workerTasks.With(emitter.workerTasksLabels[worker][key]).Set(event.Value)
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

	emitter.httpRequestsDuration.WithLabelValues(method, route, status).Observe(event.Value / 1000)
}

func (emitter *PrometheusEmitter) databaseMetrics(logger lager.Logger, event metric.Event) {
	switch event.Name {
	case "database queries":
		emitter.dbQueriesTotal.Add(event.Value)
	case "database connections":
		connectionName, exists := event.Attributes["ConnectionName"]
		if !exists {
			logger.Error("failed-to-connection-name-in-event", fmt.Errorf("expected ConnectionName to exist in event.Attributes"))
			return
		}
		emitter.dbConnections.WithLabelValues(connectionName).Set(event.Value)
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

func (emitter *PrometheusEmitter) checkQueueSizeMetric(logger lager.Logger, event metric.Event) {
	emitter.checkQueueSize.Set(event.Value)
}

func (emitter *PrometheusEmitter) checkEnqueueMetric(logger lager.Logger, event metric.Event) {
	scopeID, exists := event.Attributes["scope_id"]
	if !exists {
		logger.Error("failed-to-find-resource-config-scope-id-in-event", fmt.Errorf("expected scope_id to exist in event.Attributes"))
		return
	}

	checkName, exists := event.Attributes["check_name"]
	if !exists {
		logger.Error("failed-to-find-check-name-in-event", fmt.Errorf("expected check_name to exist in event.Attributes"))
		return
	}

	emitter.checkEnqueueVec.WithLabelValues(scopeID, checkName).Inc()
}

func (emitter *PrometheusEmitter) checkMetric(logger lager.Logger, event metric.Event) {
	scopeID, exists := event.Attributes["scope_id"]
	if !exists {
		logger.Error("failed-to-find-resource-config-scope-id-in-event", fmt.Errorf("expected scope_id to exist in event.Attributes"))
		return
	}
	checkStatus, exists := event.Attributes["check_status"]
	if !exists {
		logger.Error("failed-to-find-check-status-in-event", fmt.Errorf("expected check_status to exist in event.Attributes"))
		return
	}

	// seconds are the standard prometheus base unit for time
	duration := event.Value / 1000

	emitter.checksVec.WithLabelValues(scopeID, checkStatus).Observe(duration)
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
				DoGarbageCollection(emitter, worker)
				delete(emitter.workerLastSeen, worker)
			}
		}
		emitter.mu.Unlock()
		time.Sleep(60 * time.Second)
	}
}

// DoGarbageCollection retrieves and deletes stale metrics by their labels.
func DoGarbageCollection(emitter PrometheusGarbageCollectable, worker string) {
	for _, labels := range emitter.WorkerContainersLabels()[worker] {
		emitter.WorkerContainers().Delete(labels)
	}

	for _, labels := range emitter.WorkerVolumesLabels()[worker] {
		emitter.WorkerVolumes().Delete(labels)
	}

	for _, labels := range emitter.WorkerTasksLabels()[worker] {
		emitter.WorkerTasks().Delete(labels)
	}

	delete(emitter.WorkerContainersLabels(), worker)
	delete(emitter.WorkerVolumesLabels(), worker)
	delete(emitter.WorkerTasksLabels(), worker)
}

//go:generate counterfeiter . PrometheusGarbageCollectable
type PrometheusGarbageCollectable interface {
	WorkerContainers() *prometheus.GaugeVec
	WorkerVolumes() *prometheus.GaugeVec
	WorkerTasks() *prometheus.GaugeVec

	WorkerContainersLabels() map[string]map[string]prometheus.Labels
	WorkerVolumesLabels() map[string]map[string]prometheus.Labels
	WorkerTasksLabels() map[string]map[string]prometheus.Labels
}

func (emitter *PrometheusEmitter) WorkerContainers() *prometheus.GaugeVec {
	return emitter.workerContainers
}

func (emitter *PrometheusEmitter) WorkerVolumes() *prometheus.GaugeVec {
	return emitter.workerVolumes
}

func (emitter *PrometheusEmitter) WorkerTasks() *prometheus.GaugeVec {
	return emitter.workerTasks
}

func (emitter *PrometheusEmitter) WorkerContainersLabels() map[string]map[string]prometheus.Labels {
	return emitter.workerContainersLabels
}

func (emitter *PrometheusEmitter) WorkerVolumesLabels() map[string]map[string]prometheus.Labels {
	return emitter.workerVolumesLabels
}

func (emitter *PrometheusEmitter) WorkerTasksLabels() map[string]map[string]prometheus.Labels {
	return emitter.workerTasksLabels
}
