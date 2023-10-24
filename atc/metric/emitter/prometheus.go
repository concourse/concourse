package emitter

import (
	"fmt"
	"net"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"code.cloudfoundry.org/lager/v3"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/metric"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

type PrometheusEmitter struct {
	jobsScheduled          prometheus.Counter
	jobsScheduling         prometheus.Gauge
	jobsSchedulingDuration *prometheus.HistogramVec

	buildsStarted prometheus.Counter
	buildsRunning prometheus.Gauge

	checkBuildsStarted prometheus.Counter
	checkBuildsRunning prometheus.Gauge

	concurrentRequestsLimitHit *prometheus.CounterVec
	concurrentRequests         *prometheus.GaugeVec

	stepsWaiting         *prometheus.GaugeVec
	stepsWaitingDuration *prometheus.HistogramVec

	buildDurationsVec *prometheus.HistogramVec
	buildsAborted     prometheus.Counter
	buildsErrored     prometheus.Counter
	buildsFailed      prometheus.Counter
	buildsFinished    prometheus.Counter
	buildsFinishedVec *prometheus.CounterVec
	buildsSucceeded   prometheus.Counter

	latestCompletedBuildStatus *prometheus.GaugeVec

	gcBuildCollectorDuration                      prometheus.Histogram
	gcWorkerCollectorDuration                     prometheus.Histogram
	gcResourceCacheUseCollectorDuration           prometheus.Histogram
	gcResourceConfigCollectorDuration             prometheus.Histogram
	gcResourceCacheCollectorDuration              prometheus.Histogram
	gcResourceTaskCacheCollectorDuration          prometheus.Histogram
	gcResourceConfigCheckSessionCollectorDuration prometheus.Histogram
	gcArtifactCollectorDuration                   prometheus.Histogram
	gcContainerCollectorDuration                  prometheus.Histogram
	gcVolumeCollectorDuration                     prometheus.Histogram

	checkBuildsAborted   prometheus.Counter
	checkBuildsErrored   prometheus.Counter
	checkBuildsFailed    prometheus.Counter
	checkBuildsFinished  prometheus.Counter
	checkBuildsSucceeded prometheus.Counter

	dbConnections  *prometheus.GaugeVec
	dbQueriesTotal prometheus.Counter

	errorLogs *prometheus.CounterVec

	httpRequestsDuration *prometheus.HistogramVec

	locksHeld *prometheus.GaugeVec

	checksFinished *prometheus.CounterVec
	checksStarted  prometheus.Counter

	checksEnqueued prometheus.Counter

	volumesStreamed prometheus.Counter

	getStepCacheHits       prometheus.Counter
	streamedResourceCaches prometheus.Counter

	workerContainers                   *prometheus.GaugeVec
	workerUnknownContainers            *prometheus.GaugeVec
	workerVolumes                      *prometheus.GaugeVec
	workerUnknownVolumes               *prometheus.GaugeVec
	workerTasks                        *prometheus.GaugeVec
	workersRegistered                  *prometheus.GaugeVec
	workerOrphanedVolumesToBeCollected prometheus.Counter

	creatingContainersToBeGarbageCollected   prometheus.Counter
	createdContainersToBeGarbageCollected    prometheus.Counter
	failedContainersToBeGarbageCollected     prometheus.Counter
	destroyingContainersToBeGarbageCollected prometheus.Counter
	createdVolumesToBeGarbageCollected       prometheus.Counter
	destroyingVolumesToBeGarbageCollected    prometheus.Counter
	failedVolumesToBeGarbageCollected        prometheus.Counter

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

// rePrometheusLabelInvalid matches any invalid characters we may have in our
// emitted labels.
//
// Prometheus format:
//
//	> Label names may contain ASCII letters, numbers, as well as underscores.
//	> They must match the regex [a-zA-Z_][a-zA-Z0-9_]*. Label names beginning
//	> with __ are reserved for internal use.
//	Link: https://prometheus.io/docs/concepts/data_model/#metric-names-and-labels
var rePrometheusLabelInvalid = regexp.MustCompile(`[^a-zA-Z0-9_]+`)

// rePrometheusLabelClean ensures we clean any duplicate underscores.
var rePrometheusLabelClean = regexp.MustCompile(`__+`)

// sanitizePrometheusLabels ensures all metric labels we register with the prometheus
// emitter are sanitized to support Prometheus format. See rePrometheusLabelInvalid
// for more information.
func sanitizePrometheusLabels(labels map[string]string) map[string]string {
	newLabels := make(map[string]string)

	for k, v := range labels {
		k = rePrometheusLabelInvalid.ReplaceAllString(k, "_")
		k = strings.Trim(rePrometheusLabelClean.ReplaceAllString(k, "_"), "_")
		newLabels[k] = v
	}

	return newLabels
}

func init() {
	metric.Metrics.RegisterEmitter(&PrometheusConfig{})
}

func (config *PrometheusConfig) Description() string { return "Prometheus" }
func (config *PrometheusConfig) IsConfigured() bool {
	return config.BindPort != "" && config.BindIP != ""
}
func (config *PrometheusConfig) bind() string {
	return fmt.Sprintf("%s:%s", config.BindIP, config.BindPort)
}

func (config *PrometheusConfig) NewEmitter(attributes map[string]string) (metric.Emitter, error) {
	// ensure there are no invalid characters in label names.
	attributes = sanitizePrometheusLabels(attributes)

	// error log metrics
	errorLogs := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace:   "concourse",
			Subsystem:   "error",
			Name:        "logs",
			Help:        "Number of error logged",
			ConstLabels: attributes,
		}, []string{"message"},
	)
	prometheus.MustRegister(errorLogs)

	// lock metrics
	locksHeld := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace:   "concourse",
		Subsystem:   "locks",
		Name:        "held",
		Help:        "Database locks held",
		ConstLabels: attributes,
	}, []string{"type"})
	prometheus.MustRegister(locksHeld)

	// job metrics
	jobsScheduled := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace:   "concourse",
		Subsystem:   "jobs",
		Name:        "scheduled_total",
		Help:        "Total number of Concourse jobs scheduled.",
		ConstLabels: attributes,
	})
	prometheus.MustRegister(jobsScheduled)

	jobsScheduling := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace:   "concourse",
		Subsystem:   "jobs",
		Name:        "scheduling",
		Help:        "Number of Concourse jobs currently being scheduled.",
		ConstLabels: attributes,
	})
	prometheus.MustRegister(jobsScheduling)

	jobsSchedulingDuration := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace:   "concourse",
		Subsystem:   "jobs",
		Name:        "schedulingDuration",
		Help:        "Duration of jobs being scheduled in milliseconds",
		ConstLabels: attributes,
		Buckets:     []float64{30, 60, 180, 300, 600, 900, 1200, 1800, 2700, 3600, 7200, 18000, 36000},
	}, []string{"pipeline", "job", "job_id"})

	prometheus.MustRegister(jobsSchedulingDuration)

	// build metrics
	buildsStarted := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace:   "concourse",
		Subsystem:   "builds",
		Name:        "started_total",
		Help:        "Total number of Concourse builds started.",
		ConstLabels: attributes,
	})
	prometheus.MustRegister(buildsStarted)

	buildsRunning := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace:   "concourse",
		Subsystem:   "builds",
		Name:        "running",
		Help:        "Number of Concourse builds currently running.",
		ConstLabels: attributes,
	})
	prometheus.MustRegister(buildsRunning)

	checkBuildsStarted := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace:   "concourse",
		Subsystem:   "builds",
		Name:        "check_started_total",
		Help:        "Total number of Concourse check builds started.",
		ConstLabels: attributes,
	})
	prometheus.MustRegister(checkBuildsStarted)

	checkBuildsRunning := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace:   "concourse",
		Subsystem:   "builds",
		Name:        "check_running",
		Help:        "Number of Concourse check builds currently running.",
		ConstLabels: attributes,
	})
	prometheus.MustRegister(checkBuildsRunning)

	concurrentRequestsLimitHit := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace:   "concourse",
		Subsystem:   "concurrent_requests",
		Name:        "limit_hit_total",
		Help:        "Total number of requests rejected because the server was already serving too many concurrent requests.",
		ConstLabels: attributes,
	}, []string{"action"})
	prometheus.MustRegister(concurrentRequestsLimitHit)

	concurrentRequests := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace:   "concourse",
		Name:        "concurrent_requests",
		Help:        "Number of concurrent requests being served by endpoints that have a specified limit of concurrent requests.",
		ConstLabels: attributes,
	}, []string{"action"})
	prometheus.MustRegister(concurrentRequests)

	latestCompletedBuildStatus := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace:   "concourse",
		Subsystem:   "builds",
		Name:        "latest_completed_build_status",
		Help:        "Status of Latest Completed Build. 0=Success, 1=Failed, 2=Aborted, 3=Errored.",
		ConstLabels: attributes,
	}, []string{"jobName", "pipelineName", "teamName"})
	prometheus.MustRegister(latestCompletedBuildStatus)

	stepsWaiting := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace:   "concourse",
		Subsystem:   "steps",
		Name:        "waiting",
		Help:        "Number of Concourse build steps currently waiting.",
		ConstLabels: attributes,
	}, []string{"platform", "teamId", "teamName", "type", "workerTags"})
	prometheus.MustRegister(stepsWaiting)

	stepsWaitingDuration := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace:   "concourse",
		Subsystem:   "steps",
		Name:        "wait_duration",
		Help:        "Elapsed time waiting for execution",
		ConstLabels: attributes,
		Buckets:     []float64{10, 30, 60, 120, 300, 600, 1800, 2400, 3000, 3600},
	}, []string{"platform", "teamId", "teamName", "type", "workerTags"})
	prometheus.MustRegister(stepsWaitingDuration)

	buildsFinished := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace:   "concourse",
		Subsystem:   "builds",
		Name:        "finished_total",
		Help:        "Total number of Concourse builds finished.",
		ConstLabels: attributes,
	})
	prometheus.MustRegister(buildsFinished)

	buildsSucceeded := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace:   "concourse",
		Subsystem:   "builds",
		Name:        "succeeded_total",
		Help:        "Total number of Concourse builds succeeded.",
		ConstLabels: attributes,
	})
	prometheus.MustRegister(buildsSucceeded)

	buildsErrored := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace:   "concourse",
		Subsystem:   "builds",
		Name:        "errored_total",
		Help:        "Total number of Concourse builds errored.",
		ConstLabels: attributes,
	})
	prometheus.MustRegister(buildsErrored)

	buildsFailed := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace:   "concourse",
		Subsystem:   "builds",
		Name:        "failed_total",
		Help:        "Total number of Concourse builds failed.",
		ConstLabels: attributes,
	})
	prometheus.MustRegister(buildsFailed)

	buildsAborted := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace:   "concourse",
		Subsystem:   "builds",
		Name:        "aborted_total",
		Help:        "Total number of Concourse builds aborted.",
		ConstLabels: attributes,
	})
	prometheus.MustRegister(buildsAborted)

	buildsFinishedVec := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace:   "concourse",
			Subsystem:   "builds",
			Name:        "finished",
			Help:        "Count of builds finished across various dimensions.",
			ConstLabels: attributes,
		},
		[]string{"team", "pipeline", "job", "status"},
	)
	prometheus.MustRegister(buildsFinishedVec)

	buildDurationsVec := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace:   "concourse",
			Subsystem:   "builds",
			Name:        "duration_seconds",
			Help:        "Build time in seconds",
			ConstLabels: attributes,
			Buckets:     []float64{1, 60, 180, 300, 600, 900, 1200, 1800, 2700, 3600, 7200, 18000, 36000},
		},
		[]string{"team", "pipeline", "job"},
	)
	prometheus.MustRegister(buildDurationsVec)

	checkBuildsFinished := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace:   "concourse",
		Subsystem:   "builds",
		Name:        "check_finished_total",
		Help:        "Total number of Concourse check builds finished.",
		ConstLabels: attributes,
	})
	prometheus.MustRegister(checkBuildsFinished)

	checkBuildsSucceeded := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace:   "concourse",
		Subsystem:   "builds",
		Name:        "check_succeeded_total",
		Help:        "Total number of Concourse check builds succeeded.",
		ConstLabels: attributes,
	})
	prometheus.MustRegister(checkBuildsSucceeded)

	checkBuildsErrored := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace:   "concourse",
		Subsystem:   "builds",
		Name:        "check_errored_total",
		Help:        "Total number of Concourse check builds errored.",
		ConstLabels: attributes,
	})
	prometheus.MustRegister(checkBuildsErrored)

	checkBuildsFailed := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace:   "concourse",
		Subsystem:   "builds",
		Name:        "check_failed_total",
		Help:        "Total number of Concourse check builds failed.",
		ConstLabels: attributes,
	})
	prometheus.MustRegister(checkBuildsFailed)

	checkBuildsAborted := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace:   "concourse",
		Subsystem:   "builds",
		Name:        "check_aborted_total",
		Help:        "Total number of Concourse check builds aborted.",
		ConstLabels: attributes,
	})
	prometheus.MustRegister(checkBuildsAborted)

	// worker metrics
	workerContainers := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace:   "concourse",
			Subsystem:   "workers",
			Name:        "containers",
			Help:        "Number of containers per worker",
			ConstLabels: attributes,
		},
		[]string{"worker", "platform", "team", "tags"},
	)
	prometheus.MustRegister(workerContainers)

	workerUnknownContainers := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace:   "concourse",
			Subsystem:   "workers",
			Name:        "unknown_containers",
			Help:        "Number of unknown containers found on worker",
			ConstLabels: attributes,
		},
		[]string{"worker"},
	)
	prometheus.MustRegister(workerUnknownContainers)

	workerVolumes := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace:   "concourse",
			Subsystem:   "workers",
			Name:        "volumes",
			Help:        "Number of volumes per worker",
			ConstLabels: attributes,
		},
		[]string{"worker", "platform", "team", "tags"},
	)
	prometheus.MustRegister(workerVolumes)

	workerUnknownVolumes := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace:   "concourse",
			Subsystem:   "workers",
			Name:        "unknown_volumes",
			Help:        "Number of unknown volumes found on worker",
			ConstLabels: attributes,
		},
		[]string{"worker"},
	)
	prometheus.MustRegister(workerUnknownVolumes)

	workerTasks := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace:   "concourse",
			Subsystem:   "workers",
			Name:        "tasks",
			Help:        "Number of active tasks per worker",
			ConstLabels: attributes,
		},
		[]string{"worker", "platform"},
	)
	prometheus.MustRegister(workerTasks)

	workersRegistered := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace:   "concourse",
			Subsystem:   "workers",
			Name:        "registered",
			Help:        "Number of workers per state as seen by the database",
			ConstLabels: attributes,
		},
		[]string{"state"},
	)
	prometheus.MustRegister(workersRegistered)

	// http metrics
	httpRequestsDuration := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace:   "concourse",
			Subsystem:   "http_responses",
			Name:        "duration_seconds",
			Help:        "Response time in seconds",
			ConstLabels: attributes,
		},
		[]string{"method", "route", "status"},
	)
	prometheus.MustRegister(httpRequestsDuration)

	dbQueriesTotal := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace:   "concourse",
		Subsystem:   "db",
		Name:        "queries_total",
		Help:        "Total number of database Concourse database queries",
		ConstLabels: attributes,
	})
	prometheus.MustRegister(dbQueriesTotal)

	dbConnections := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace:   "concourse",
			Subsystem:   "db",
			Name:        "connections",
			Help:        "Current number of concourse database connections",
			ConstLabels: attributes,
		},
		[]string{"dbname"},
	)
	prometheus.MustRegister(dbConnections)

	checksFinished := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace:   "concourse",
			Subsystem:   "lidar",
			Name:        "checks_finished_total",
			Help:        "Total number of checks finished.",
			ConstLabels: attributes,
		},
		[]string{"status"},
	)
	prometheus.MustRegister(checksFinished)

	checksStarted := prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace:   "concourse",
			Subsystem:   "lidar",
			Name:        "checks_started_total",
			Help:        "Total number of checks started. With global resource enabled, a check build may not really run a check, thus total checks started should be less than total check builds started.",
			ConstLabels: attributes,
		},
	)
	prometheus.MustRegister(checksStarted)

	checksEnqueued := prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace:   "concourse",
			Subsystem:   "lidar",
			Name:        "checks_enqueued_total",
			Help:        "Total number of checks enqueued",
			ConstLabels: attributes,
		},
	)
	prometheus.MustRegister(checksEnqueued)

	volumesStreamed := prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace:   "concourse",
			Subsystem:   "volumes",
			Name:        "volumes_streamed",
			Help:        "Total number of volumes streamed from one worker to the other",
			ConstLabels: attributes,
		},
	)
	prometheus.MustRegister(volumesStreamed)

	workerOrphanedVolumesToBeCollected := prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace:   "concourse",
			Subsystem:   "volumes",
			Name:        "orphaned_volumes_to_be_deleted",
			Help:        "Number of orphaned volumes to be garbage collected.",
			ConstLabels: attributes,
		},
	)
	prometheus.MustRegister(workerOrphanedVolumesToBeCollected)

	creatingContainersToBeGarbageCollected := prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace:   "concourse",
			Subsystem:   "gc",
			Name:        "creating_containers_to_be_garbage_collected",
			Help:        "Creating Containers being garbage collected",
			ConstLabels: attributes,
		},
	)
	prometheus.MustRegister(creatingContainersToBeGarbageCollected)

	createdContainersToBeGarbageCollected := prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace:   "concourse",
			Subsystem:   "gc",
			Name:        "created_containers_to_be_garbage_collected",
			Help:        "Created Containers being garbage collected",
			ConstLabels: attributes,
		},
	)
	prometheus.MustRegister(createdContainersToBeGarbageCollected)

	failedContainersToBeGarbageCollected := prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace:   "concourse",
			Subsystem:   "gc",
			Name:        "failed_containers_to_be_garbage_collected",
			Help:        "Failed Containers being garbage collected",
			ConstLabels: attributes,
		},
	)
	prometheus.MustRegister(failedContainersToBeGarbageCollected)

	destroyingContainersToBeGarbageCollected := prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace:   "concourse",
			Subsystem:   "gc",
			Name:        "destroying_containers_to_be_garbage_collected",
			Help:        "Destorying Containers being garbage collected",
			ConstLabels: attributes,
		},
	)
	prometheus.MustRegister(destroyingContainersToBeGarbageCollected)

	createdVolumesToBeGarbageCollected := prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace:   "concourse",
			Subsystem:   "gc",
			Name:        "created_volumes_to_be_garbage_collected",
			Help:        "Created Volumes being garbage collected",
			ConstLabels: attributes,
		},
	)
	prometheus.MustRegister(createdVolumesToBeGarbageCollected)

	destroyingVolumesToBeGarbageCollected := prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace:   "concourse",
			Subsystem:   "gc",
			Name:        "destroying_volumes_to_be_garbage_collected",
			Help:        "Destroying Volumes being garbage collected",
			ConstLabels: attributes,
		},
	)
	prometheus.MustRegister(destroyingVolumesToBeGarbageCollected)

	failedVolumesToBeGarbageCollected := prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace:   "concourse",
			Subsystem:   "gc",
			Name:        "failed_volumes_to_be_garbage_collected",
			Help:        "Failed Volumes being garbage collected",
			ConstLabels: attributes,
		},
	)
	prometheus.MustRegister(failedVolumesToBeGarbageCollected)

	gcBuildCollectorDuration := prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Namespace:   "concourse",
			Subsystem:   "gc",
			Name:        "gc_build_collector_duration",
			Help:        "Duration of gc build collector (ms)",
			ConstLabels: attributes,
			Buckets:     []float64{1, 60, 180, 300, 600, 900, 1200, 1800, 2700, 3600, 7200, 18000, 36000},
		},
	)
	prometheus.MustRegister(gcBuildCollectorDuration)

	gcWorkerCollectorDuration := prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Namespace:   "concourse",
			Subsystem:   "gc",
			Name:        "gc_worker_collector_duration",
			Help:        "Duration of gc worker collector (ms)",
			ConstLabels: attributes,
			Buckets:     []float64{1, 60, 180, 300, 600, 900, 1200, 1800, 2700, 3600, 7200, 18000, 36000},
		},
	)
	prometheus.MustRegister(gcWorkerCollectorDuration)

	gcResourceCacheUseCollectorDuration := prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Namespace:   "concourse",
			Subsystem:   "gc",
			Name:        "gc_resource_cache_use_collector_duration",
			Help:        "Duration of gc resource cache use collector (ms)",
			ConstLabels: attributes,
			Buckets:     []float64{1, 60, 180, 300, 600, 900, 1200, 1800, 2700, 3600, 7200, 18000, 36000},
		},
	)
	prometheus.MustRegister(gcResourceCacheUseCollectorDuration)

	gcResourceConfigCollectorDuration := prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Namespace:   "concourse",
			Subsystem:   "gc",
			Name:        "gc_resource_config_collector_duration",
			Help:        "Duration of gc resource config collector (ms)",
			ConstLabels: attributes,
			Buckets:     []float64{1, 60, 180, 300, 600, 900, 1200, 1800, 2700, 3600, 7200, 18000, 36000},
		},
	)
	prometheus.MustRegister(gcResourceConfigCollectorDuration)

	gcResourceCacheCollectorDuration := prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Namespace:   "concourse",
			Subsystem:   "gc",
			Name:        "gc_resource_cache_collector_duration",
			Help:        "Duration of gc resource cache collector (ms)",
			ConstLabels: attributes,
			Buckets:     []float64{1, 60, 180, 300, 600, 900, 1200, 1800, 2700, 3600, 7200, 18000, 36000},
		},
	)
	prometheus.MustRegister(gcResourceCacheCollectorDuration)

	gcResourceTaskCacheCollectorDuration := prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Namespace:   "concourse",
			Subsystem:   "gc",
			Name:        "gc_task_cache_collector_duration",
			Help:        "Duration of gc task cache collector (ms)",
			ConstLabels: attributes,
			Buckets:     []float64{1, 60, 180, 300, 600, 900, 1200, 1800, 2700, 3600, 7200, 18000, 36000},
		},
	)
	prometheus.MustRegister(gcResourceTaskCacheCollectorDuration)

	gcResourceConfigCheckSessionCollectorDuration := prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Namespace:   "concourse",
			Subsystem:   "gc",
			Name:        "gc_resource_config_check_session_collector_duration",
			Help:        "Duration of gc resource config check session collector (ms)",
			ConstLabels: attributes,
			Buckets:     []float64{1, 60, 180, 300, 600, 900, 1200, 1800, 2700, 3600, 7200, 18000, 36000},
		},
	)
	prometheus.MustRegister(gcResourceConfigCheckSessionCollectorDuration)

	gcArtifactCollectorDuration := prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Namespace:   "concourse",
			Subsystem:   "gc",
			Name:        "gc_artifact_collector_duration",
			Help:        "Duration of gc artifact collector (ms)",
			ConstLabels: attributes,
			Buckets:     []float64{1, 60, 180, 300, 600, 900, 1200, 1800, 2700, 3600, 7200, 18000, 36000},
		},
	)
	prometheus.MustRegister(gcArtifactCollectorDuration)

	gcContainerCollectorDuration := prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Namespace:   "concourse",
			Subsystem:   "gc",
			Name:        "gc_container_collector_duration",
			Help:        "Duration of gc container collector (ms)",
			ConstLabels: attributes,
			Buckets:     []float64{1, 60, 180, 300, 600, 900, 1200, 1800, 2700, 3600, 7200, 18000, 36000},
		},
	)
	prometheus.MustRegister(gcContainerCollectorDuration)

	gcVolumeCollectorDuration := prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Namespace:   "concourse",
			Subsystem:   "gc",
			Name:        "gc_volume_collector_duration",
			Help:        "Duration of gc volume collector (ms)",
			ConstLabels: attributes,
			Buckets:     []float64{1, 60, 180, 300, 600, 900, 1200, 1800, 2700, 3600, 7200, 18000, 36000},
		},
	)
	prometheus.MustRegister(gcVolumeCollectorDuration)

	getStepCacheHits := prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace:   "concourse",
			Subsystem:   "caches",
			Name:        "get_step_cache_hits",
			Help:        "Total number of get steps that hit caches",
			ConstLabels: attributes,
		},
	)
	prometheus.MustRegister(getStepCacheHits)

	streamedResourceCaches := prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace:   "concourse",
			Subsystem:   "caches",
			Name:        "streamed_resource_caches",
			Help:        "Total number of streamed resource caches",
			ConstLabels: attributes,
		},
	)
	prometheus.MustRegister(streamedResourceCaches)

	listener, err := net.Listen("tcp", config.bind())
	if err != nil {
		return nil, err
	}

	go http.Serve(listener, promhttp.Handler())

	emitter := &PrometheusEmitter{
		jobsScheduled:          jobsScheduled,
		jobsScheduling:         jobsScheduling,
		jobsSchedulingDuration: jobsSchedulingDuration,

		buildsStarted: buildsStarted,
		buildsRunning: buildsRunning,

		checkBuildsStarted: checkBuildsStarted,
		checkBuildsRunning: checkBuildsRunning,

		concurrentRequestsLimitHit: concurrentRequestsLimitHit,
		concurrentRequests:         concurrentRequests,

		latestCompletedBuildStatus:            latestCompletedBuildStatus,
		stepsWaiting:         stepsWaiting,
		stepsWaitingDuration: stepsWaitingDuration,

		creatingContainersToBeGarbageCollected:   creatingContainersToBeGarbageCollected,
		createdContainersToBeGarbageCollected:    createdContainersToBeGarbageCollected,
		failedContainersToBeGarbageCollected:     failedContainersToBeGarbageCollected,
		destroyingContainersToBeGarbageCollected: destroyingContainersToBeGarbageCollected,
		createdVolumesToBeGarbageCollected:       createdVolumesToBeGarbageCollected,
		destroyingVolumesToBeGarbageCollected:    destroyingVolumesToBeGarbageCollected,
		failedVolumesToBeGarbageCollected:        failedVolumesToBeGarbageCollected,

		gcBuildCollectorDuration:                      gcBuildCollectorDuration,
		gcWorkerCollectorDuration:                     gcWorkerCollectorDuration,
		gcResourceCacheUseCollectorDuration:           gcResourceCacheUseCollectorDuration,
		gcResourceConfigCollectorDuration:             gcResourceConfigCollectorDuration,
		gcResourceCacheCollectorDuration:              gcResourceCacheCollectorDuration,
		gcResourceTaskCacheCollectorDuration:          gcResourceTaskCacheCollectorDuration,
		gcResourceConfigCheckSessionCollectorDuration: gcResourceConfigCheckSessionCollectorDuration,
		gcArtifactCollectorDuration:                   gcArtifactCollectorDuration,
		gcContainerCollectorDuration:                  gcContainerCollectorDuration,
		gcVolumeCollectorDuration:                     gcVolumeCollectorDuration,

		buildDurationsVec: buildDurationsVec,
		buildsAborted:     buildsAborted,
		buildsErrored:     buildsErrored,
		buildsFailed:      buildsFailed,
		buildsFinished:    buildsFinished,
		buildsFinishedVec: buildsFinishedVec,
		buildsSucceeded:   buildsSucceeded,

		checkBuildsAborted:   checkBuildsAborted,
		checkBuildsErrored:   checkBuildsErrored,
		checkBuildsFailed:    checkBuildsFailed,
		checkBuildsFinished:  checkBuildsFinished,
		checkBuildsSucceeded: checkBuildsSucceeded,

		dbConnections:  dbConnections,
		dbQueriesTotal: dbQueriesTotal,

		errorLogs: errorLogs,

		httpRequestsDuration: httpRequestsDuration,

		locksHeld: locksHeld,

		checksFinished: checksFinished,
		checksStarted:  checksStarted,

		checksEnqueued: checksEnqueued,

		workerContainers:                   workerContainers,
		workersRegistered:                  workersRegistered,
		workerContainersLabels:             map[string]map[string]prometheus.Labels{},
		workerVolumesLabels:                map[string]map[string]prometheus.Labels{},
		workerTasksLabels:                  map[string]map[string]prometheus.Labels{},
		workerLastSeen:                     map[string]time.Time{},
		workerVolumes:                      workerVolumes,
		workerTasks:                        workerTasks,
		workerUnknownContainers:            workerUnknownContainers,
		workerUnknownVolumes:               workerUnknownVolumes,
		workerOrphanedVolumesToBeCollected: workerOrphanedVolumesToBeCollected,

		volumesStreamed: volumesStreamed,

		getStepCacheHits:       getStepCacheHits,
		streamedResourceCaches: streamedResourceCaches,
	}
	go emitter.periodicMetricGC()

	return emitter, nil
}

// Emit processes incoming metrics.
// In order to provide idiomatic Prometheus metrics, we'll have to convert the various
// Event types (differentiated by the less-than-ideal string Name field) into different
// Prometheus metrics.
func (emitter *PrometheusEmitter) Emit(logger lager.Logger, event metric.Event) {
	// ensure there are no invalid characters in label names.
	event.Attributes = sanitizePrometheusLabels(event.Attributes)

	switch event.Name {
	case "error log":
		emitter.errorLogsMetric(logger, event)
	case "lock held":
		emitter.lock(logger, event)
	case "jobs scheduled":
		emitter.jobsScheduled.Add(event.Value)
	case "jobs scheduling":
		emitter.jobsScheduling.Set(event.Value)
	case "scheduling: job duration (ms)":
		emitter.jobsSchedulingDuration.WithLabelValues(
			event.Attributes["pipeline"],
			event.Attributes["job"],
			event.Attributes["job_id"],
		).Observe(event.Value)
	case "builds started":
		emitter.buildsStarted.Add(event.Value)
	case "builds running":
		emitter.buildsRunning.Set(event.Value)
	case "check builds started":
		emitter.checkBuildsStarted.Add(event.Value)
	case "check builds running":
		emitter.checkBuildsRunning.Set(event.Value)
	case "concurrent requests limit hit":
		emitter.concurrentRequestsLimitHit.WithLabelValues(event.Attributes["action"]).Add(event.Value)
	case "concurrent requests":
		emitter.concurrentRequests.
			WithLabelValues(event.Attributes["action"]).Set(event.Value)
	case "latest completed build status":
		emitter.latestCompletedBuildStatus.
			WithLabelValues(
				event.Attributes["jobName"],
				event.Attributes["pipelineName"],
				event.Attributes["teamName"],
			).Set(event.Value)
	case "steps waiting":
		emitter.stepsWaiting.
			WithLabelValues(
				event.Attributes["platform"],
				event.Attributes["teamId"],
				event.Attributes["teamName"],
				event.Attributes["type"],
				event.Attributes["workerTags"],
			).Set(event.Value)
	case "steps waiting duration":
		emitter.stepsWaitingDuration.
			WithLabelValues(
				event.Attributes["platform"],
				event.Attributes["teamId"],
				event.Attributes["teamName"],
				event.Attributes["type"],
				event.Attributes["workerTags"],
			).Observe(event.Value)
	case "build finished":
		emitter.buildFinishedMetrics(logger, event)
	case "worker containers":
		// update last seen counters, used to gc stale timeseries
		emitter.updateLastSeen(event)
		emitter.workerContainersMetric(logger, event)
	case "creating containers to be garbage collected":
		emitter.creatingContainersToBeGarbageCollected.Add(event.Value)
	case "created containers to be garbage collected":
		emitter.createdContainersToBeGarbageCollected.Add(event.Value)
	case "failed containers to be garbage collected":
		emitter.failedContainersToBeGarbageCollected.Add(event.Value)
	case "destroying containers to be garbage collected":
		emitter.destroyingContainersToBeGarbageCollected.Add(event.Value)
	case "created volumes to be garbage collected":
		emitter.createdVolumesToBeGarbageCollected.Add(event.Value)
	case "destroying volumes to be garbage collected":
		emitter.destroyingVolumesToBeGarbageCollected.Add(event.Value)
	case "failed volumes to be garbage collected":
		emitter.failedVolumesToBeGarbageCollected.Add(event.Value)
	case "worker volumes":
		// update last seen counters, used to gc stale timeseries
		emitter.updateLastSeen(event)
		emitter.workerVolumesMetric(logger, event)
	case "worker unknown containers":
		emitter.workerUnknownContainersMetric(logger, event)
	case "worker unknown volumes":
		emitter.workerUnknownVolumesMetric(logger, event)
	case "worker tasks":
		// update last seen counters, used to gc stale timeseries
		emitter.updateLastSeen(event)
		emitter.workerTasksMetric(logger, event)
	case "worker state":
		emitter.workersRegisteredMetric(logger, event)
	case "orphaned volumes to be garbage collected":
		emitter.workerOrphanedVolumesToBeCollected.Add(event.Value)
	case "gc: build collector duration (ms)":
		emitter.gcBuildCollectorDuration.Observe(event.Value)
	case "gc: worker collector duration (ms)":
		emitter.gcWorkerCollectorDuration.Observe(event.Value)
	case "gc: resource cache use collector duration (ms)":
		emitter.gcResourceCacheUseCollectorDuration.Observe(event.Value)
	case "gc: resource config collector duration (ms)":
		emitter.gcResourceConfigCollectorDuration.Observe(event.Value)
	case "gc: resource cache collector duration (ms)":
		emitter.gcResourceCacheCollectorDuration.Observe(event.Value)
	case "gc: task cache collector duration (ms)":
		emitter.gcResourceTaskCacheCollectorDuration.Observe(event.Value)
	case "gc: resource config check session collector duration (ms)":
		emitter.gcResourceConfigCheckSessionCollectorDuration.Observe(event.Value)
	case "gc: artifact collector duration (ms)":
		emitter.gcArtifactCollectorDuration.Observe(event.Value)
	case "gc: container collector duration (ms)":
		emitter.gcContainerCollectorDuration.Observe(event.Value)
	case "gc: volume collector duration (ms)":
		emitter.gcVolumeCollectorDuration.Observe(event.Value)
	case "http response time":
		emitter.httpResponseTimeMetrics(logger, event)
	case "database queries":
		emitter.databaseMetrics(logger, event)
	case "database connections":
		emitter.databaseMetrics(logger, event)
	case "checks finished":
		emitter.checksFinished.WithLabelValues(event.Attributes["status"]).Add(event.Value)
	case "checks started":
		emitter.checksStarted.Add(event.Value)
	case "checks enqueued":
		emitter.checksEnqueued.Add(event.Value)
	case "volumes streamed":
		emitter.volumesStreamed.Add(event.Value)
	case "get step cache hits":
		emitter.getStepCacheHits.Add(event.Value)
	case "streamed resource caches":
		emitter.streamedResourceCaches.Add(event.Value)
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
			fmt.Errorf("expected message to exist in event.Attributes"))
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

func (emitter *PrometheusEmitter) checkBuildFinishedMetrics(logger lager.Logger, event metric.Event) {
	// concourse_builds_finished_total
	emitter.checkBuildsFinished.Inc()

	buildStatus, exists := event.Attributes["build_status"]
	if !exists {
		logger.Error("failed-to-find-check-build_status-in-event", fmt.Errorf("expected build_status to exist in event.Attributes"))
		return
	}

	// concourse_builds_(aborted|succeeded|failed|errored)_total
	switch buildStatus {
	case string(db.BuildStatusAborted):
		// concourse_builds_check_aborted_total
		emitter.checkBuildsAborted.Inc()
	case string(db.BuildStatusSucceeded):
		// concourse_builds_check_succeeded_total
		emitter.checkBuildsSucceeded.Inc()
	case string(db.BuildStatusFailed):
		// concourse_builds_check_failed_total
		emitter.checkBuildsFailed.Inc()
	case string(db.BuildStatusErrored):
		// concourse_builds_check_errored_total
		emitter.checkBuildsErrored.Inc()
	}
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
	tags := event.Attributes["tags"]

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
	tags := event.Attributes["tags"]

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

// updateLastSeen tracks for each worker when it last received a metric event.
func (emitter *PrometheusEmitter) updateLastSeen(event metric.Event) {
	emitter.mu.Lock()
	defer emitter.mu.Unlock()
	if worker, exists := event.Attributes["worker"]; exists {
		emitter.workerLastSeen[worker] = time.Now()
	}
}

// periodically remove stale metrics for workers
func (emitter *PrometheusEmitter) periodicMetricGC() {
	for {
		emitter.mu.Lock()
		now := time.Now()
		for worker, lastSeen := range emitter.workerLastSeen {
			if now.Sub(lastSeen) > 2*time.Minute {
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

//counterfeiter:generate . PrometheusGarbageCollectable
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
