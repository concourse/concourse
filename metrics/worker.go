package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	VolumesSweepingDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "concourse_worker_volumes_sweeping_duration_seconds",
			Help: "Time taken to sweep a volume",
		},
		[]string{LabelStatus},
	)

	Volumes = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "concourse_worker_volumes",
			Help: "Number of volumes",
		},
	)
)

var (
	ContainersSweepingDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "concourse_worker_containers_sweeping_duration_seconds",
			Help: "Time taken to sweep a container",
		},
		[]string{LabelStatus},
	)

	Containers = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "concourse_worker_containers",
			Help: "Number of containers",
		},
	)
)

func NewWorkerMetricsHandler() http.Handler {
	registry := prometheus.NewRegistry()

	for _, collector := range defaultCollectors {
		registry.MustRegister(collector)
	}

	registry.MustRegister(
		VolumesSweepingDuration,
		Volumes,

		ContainersSweepingDuration,
		Containers,
	)

	return promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
}
