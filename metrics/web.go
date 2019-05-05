package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	slowBuckets = []float64{
		1, 30, 60, 120, 180, 300, 600, 900, 1200, 1800, 2700, 3600, 7200, 18000, 36000,
	}
)

var (
	HttpResponseDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "concourse_http_response_duration_seconds",
			Help: "How long requests are taking to be served.",
		},
		[]string{"code", "route"},
	)
)

var (
	SchedulingDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "concourse_scheduling_duration_seconds",
			Help: "How long it took for a full scheduling tick pipeline.",
		},
		[]string{LabelStatus},
	)
)

var (
	ContainersCreationDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "concourse_containers_creation_duration_seconds",
			Help: "Time taken to create a container",
		},
		[]string{LabelStatus},
	)
	ContainersToBeGCed = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "concourse_gc_containers_to_be_gced_total",
			Help: "Number of containers found for deletion",
		},
		[]string{"type"},
	)
	ContainersGCed = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "concourse_gc_containers_gced_total",
			Help: "Number containers actually deleted",
		},
	)
)

var (
	VolumesCreationDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "concourse_volumes_creation_duration_seconds",
			Help: "Time taken to create a container",
		},
		[]string{LabelStatus},
	)
	VolumesToBeGCed = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "concourse_gc_volumes_to_be_gced_total",
			Help: "Number of volumes found for deletion",
		},
		[]string{"type"},
	)
	VolumesGCed = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "concourse_gc_volumes_gced_total",
			Help: "Number volumes actually deleted",
		},
		[]string{"type"},
	)
)

var (
	BuildsDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "concourse_builds_duration_seconds",
			Help:    "How long it took for builds to finish",
			Buckets: slowBuckets,
		},
		[]string{LabelStatus, "team", "pipeline", "job"},
	)
)

var (
	ResourceChecksDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "concourse_resource_checks_duration_seconds",
			Help: "How long resource checks take",
		},
		[]string{LabelStatus},
	)
)

var (
	DatabaseQueries = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "concourse_db_queries_total",
			Help: "Number of queries performed",
		},
	)
)

func NewWebMetricsHandler() http.Handler {
	registry := prometheus.NewRegistry()

	for _, collector := range defaultCollectors {
		registry.MustRegister(collector)
	}

	registry.MustRegister(
		HttpResponseDuration,

		SchedulingDuration,

		ContainersCreationDuration,
		ContainersToBeGCed,
		ContainersGCed,

		VolumesCreationDuration,
		VolumesToBeGCed,
		VolumesGCed,

		BuildsDuration,

		ResourceChecksDuration,

		DatabaseQueries,
	)

	return promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
}
