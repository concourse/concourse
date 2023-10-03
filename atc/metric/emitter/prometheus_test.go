package emitter_test

import (
	"fmt"
	"io"
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/lager/v3/lagertest"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/concourse/concourse/atc/metric"
	"github.com/concourse/concourse/atc/metric/emitter"
	"github.com/concourse/concourse/atc/metric/emitter/emitterfakes"
)

var _ = Describe("PrometheusEmitter garbage collector", func() {
	var (
		fake emitterfakes.FakePrometheusGarbageCollectable

		labelsLong  prometheus.Labels
		labelsShort prometheus.Labels

		workerContainers *prometheus.GaugeVec
		workerVolumes    *prometheus.GaugeVec
		workerTasks      *prometheus.GaugeVec

		workerContainersLabels map[string]map[string]prometheus.Labels
		workerVolumesLabels    map[string]map[string]prometheus.Labels
		workerTasksLabels      map[string]map[string]prometheus.Labels
	)

	BeforeEach(func() {
		workerContainers = prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "concoursedev",
				Subsystem: "workers",
				Name:      "containers",
				Help:      "Number of containers per worker",
			},
			[]string{"worker", "platform", "team", "tags"},
		)
		prometheus.Register(workerContainers)

		workerVolumes = prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "concoursedev",
				Subsystem: "workers",
				Name:      "volumes",
				Help:      "Number of volumes per worker",
			},
			[]string{"worker", "platform", "team", "tags"},
		)
		prometheus.Register(workerVolumes)

		workerTasks = prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "concoursedev",
				Subsystem: "workers",
				Name:      "tasks",
				Help:      "Number of active tasks per worker",
			},
			[]string{"worker", "platform"},
		)
		prometheus.Register(workerTasks)

		workerContainersLabels = map[string]map[string]prometheus.Labels{}
		workerVolumesLabels = map[string]map[string]prometheus.Labels{}
		workerTasksLabels = map[string]map[string]prometheus.Labels{}

		labelsLong = prometheus.Labels{
			"worker":   "foo",
			"platform": "linux",
			"team":     "main",
			"tags":     "",
		}

		labelsShort = prometheus.Labels{
			"worker":   "foo",
			"platform": "linux",
		}
	})
	JustBeforeEach(func() {
		fake = emitterfakes.FakePrometheusGarbageCollectable{
			WorkerContainersStub: func() *prometheus.GaugeVec { return workerContainers },
			WorkerVolumesStub:    func() *prometheus.GaugeVec { return workerVolumes },
			WorkerTasksStub:      func() *prometheus.GaugeVec { return workerTasks },

			WorkerContainersLabelsStub: func() map[string]map[string]prometheus.Labels {
				return workerContainersLabels
			},
			WorkerVolumesLabelsStub: func() map[string]map[string]prometheus.Labels {
				return workerVolumesLabels
			},
			WorkerTasksLabelsStub: func() map[string]map[string]prometheus.Labels {
				return workerTasksLabels
			},
		}

		// Deep copy the labels so we can use them to verify the test results later
		labels := make(prometheus.Labels)
		for k, v := range labelsLong {
			labels[k] = v
		}
		fake.WorkerContainers().With(labels).Set(42.0)
		fake.WorkerContainersLabels()["foo"] = make(map[string]prometheus.Labels)
		fake.WorkerContainersLabels()["foo"]["foo_linux_main__"] = labels

		fake.WorkerVolumes().With(labels).Set(42.0)
		fake.WorkerVolumesLabels()["foo"] = make(map[string]prometheus.Labels)
		fake.WorkerVolumesLabels()["foo"]["foo_linux_main__"] = labels

		labels = make(prometheus.Labels)
		for k, v := range labelsShort {
			labels[k] = v
		}
		fake.WorkerTasks().With(labels).Set(42.0)
		fake.WorkerTasksLabels()["foo"] = make(map[string]prometheus.Labels)
		fake.WorkerTasksLabels()["foo"]["foo_linux"] = labels
	})

	It("should remove all metrics from the emitter", func() {
		Expect(fake.WorkerContainersLabels()).To(HaveLen(1))
		Expect(fake.WorkerVolumesLabels()).To(HaveLen(1))
		Expect(fake.WorkerTasksLabels()).To(HaveLen(1))

		emitter.DoGarbageCollection(&fake, "foo")

		Expect(fake.WorkerContainersLabels()).To(HaveLen(0))
		Expect(fake.WorkerVolumesLabels()).To(HaveLen(0))
		Expect(fake.WorkerTasksLabels()).To(HaveLen(0))

		// Delete should return false if the metrics no longer exist
		Expect(fake.WorkerContainers().Delete(labelsLong)).To(Equal(false))
		Expect(fake.WorkerVolumes().Delete(labelsLong)).To(Equal(false))
		Expect(fake.WorkerTasks().Delete(labelsShort)).To(Equal(false))
	})

	// There is no easy way to detect whether metrics are REALLY garbage collected due to the
	// limitations of the Prometheus client library, so here we verify that the metrics that were
	// deleted in the previous spec were actually present from the beginning.
	It("should not do anything if there are no metrics", func() {
		// Delete should return true if the metrics are actually deleted
		Expect(fake.WorkerContainers().Delete(labelsLong)).To(Equal(true))
		Expect(fake.WorkerVolumes().Delete(labelsLong)).To(Equal(true))
		Expect(fake.WorkerTasks().Delete(labelsShort)).To(Equal(true))

		emitter.DoGarbageCollection(&fake, "foo")

		// Delete should return false if the metrics no longer exist
		Expect(fake.WorkerContainers().Delete(labelsLong)).To(Equal(false))
		Expect(fake.WorkerVolumes().Delete(labelsLong)).To(Equal(false))
		Expect(fake.WorkerTasks().Delete(labelsShort)).To(Equal(false))

	})

	AfterEach(func() {
		workerContainers.Reset()
		workerVolumes.Reset()
		workerTasks.Reset()

		workerContainersLabels = map[string]map[string]prometheus.Labels{}
		workerVolumesLabels = map[string]map[string]prometheus.Labels{}
		workerTasksLabels = map[string]map[string]prometheus.Labels{}

		prometheus.Unregister(workerContainers)
		prometheus.Unregister(workerVolumes)
		prometheus.Unregister(workerTasks)
	})
})

var _ = Describe("PrometheusEmitter", func() {
	var (
		prometheusConfig  *emitter.PrometheusConfig
		prometheusEmitter metric.Emitter
		logger            *lagertest.TestLogger
		err               error
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test")
		prometheusConfig = &emitter.PrometheusConfig{
			BindIP:   "localhost",
			BindPort: "9090",
		}
	})

	JustBeforeEach(func() {
		prometheusEmitter, err = prometheusConfig.NewEmitter(map[string]string{
			// Ensure invalid labels are sanitized.
			"invalid-label":     "foo",
			"__prefix__test__":  "bar",
			"_prefix_testtwo__": "baz",
		})
		Expect(err).To(BeNil())
	})

	It("emits metric", func() {
		prometheusEmitter.Emit(logger, metric.Event{
			Name:  "steps waiting",
			Value: 4,
			Attributes: map[string]string{
				"platform":   "darwin",
				"teamId":     "42",
				"teamName":   "teamdev",
				"type":       "get",
				"workerTags": "tester",
			},
		})

		prometheusEmitter.Emit(logger, metric.Event{
			Name:  "latest completed build status",
			Value: 0,
			Attributes: map[string]string{
				"jobName":      "job1",
				"pipelineName": "pipeline1",
				"teamName":     "team1",
			},
		})

		getPrometheusMetrics := func() string {
			res, _ := http.Get(fmt.Sprintf("http://%s:%s/metrics", prometheusConfig.BindIP, prometheusConfig.BindPort))
			body, _ := io.ReadAll(res.Body)
			defer res.Body.Close()

			Expect(res.StatusCode).To(Equal(http.StatusOK))
			return string(body)
		}

		Eventually(getPrometheusMetrics()).Should(ContainSubstring("concourse_steps_waiting{invalid_label=\"foo\",platform=\"darwin\",prefix_test=\"bar\",prefix_testtwo=\"baz\",teamId=\"42\",teamName=\"teamdev\",type=\"get\",workerTags=\"tester\"} 4"))
		Eventually(getPrometheusMetrics()).Should(ContainSubstring("concourse_builds_latest_completed_build_status{invalid_label=\"foo\",jobName=\"job1\",pipelineName=\"pipeline1\",prefix_test=\"bar\",prefix_testtwo=\"baz\",teamName=\"team1\"} 0"))
	})
})
