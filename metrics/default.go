package metrics

import (
	"github.com/concourse/concourse"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	version = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "concourse_version",
			Help: "Concourse version",
		},
		[]string{"version", "worker_version"},
	)
)

func init() {
	version.
		WithLabelValues(concourse.Version, concourse.WorkerVersion).
		Inc()
}

var (
	defaultCollectors = []prometheus.Collector{
		prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}),
		prometheus.NewGoCollector(),
		version,
	}
)
