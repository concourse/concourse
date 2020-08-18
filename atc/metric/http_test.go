package metric_test

import (
	"net/http"
	"net/http/httptest"

	"github.com/concourse/concourse/atc/metric"
	"github.com/concourse/concourse/atc/metric/metricfakes"

	. "github.com/concourse/concourse/atc/metric"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func noopHandler(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/success":
		return
	case "/failure":
		w.WriteHeader(500)
		return
	}

	w.WriteHeader(404)
	return
}

var _ = Describe("MetricsHandler", func() {
	var (
		ts      *httptest.Server
		emitter *metricfakes.FakeEmitter
		monitor *metric.Monitor
	)

	BeforeEach(func() {
		emitter = &metricfakes.FakeEmitter{}
		monitor = metric.NewMonitor()

		emitterFactory := &metricfakes.FakeEmitterFactory{}
		emitterFactory.IsConfiguredReturns(true)
		emitterFactory.NewEmitterReturns(emitter, nil)

		monitor.RegisterEmitter(emitterFactory)
		monitor.Initialize(testLogger, "test", map[string]string{}, 1000)

		ts = httptest.NewServer(
			WrapHandler(
				testLogger,
				monitor,
				"ApiEndpoint",
				http.HandlerFunc(noopHandler),
			),
		)
	})

	AfterEach(func() {
		ts.Close()
	})

	Context("when serving requests", func() {
		var (
			endpoint = "/"
			event    metric.Event
		)

		JustBeforeEach(func() {
			res, err := http.Get(ts.URL + endpoint)
			Expect(err).ToNot(HaveOccurred())
			res.Body.Close()

			Eventually(emitter.EmitCallCount).Should(BeNumerically("==", 1))
			_, event = emitter.EmitArgsForCall(0)
		})

		It("captures request and response properties", func() {
			Expect(event.Attributes).To(HaveKeyWithValue("status", "404"))
			Expect(event.Attributes).To(HaveKeyWithValue("method", "GET"))
			Expect(event.Attributes).To(HaveKeyWithValue("route", "ApiEndpoint"))
			Expect(event.Attributes).To(HaveKeyWithValue("path", "/"))
		})

		Context("to endpoint that returns success statuses", func() {
			BeforeEach(func() {
				endpoint = "/success"
			})

			It("captures error code", func() {
				Expect(event.Attributes).To(HaveKeyWithValue("status", "200"))
			})

			It("captures route", func() {
				Expect(event.Attributes).To(HaveKeyWithValue("path", "/success"))
			})
		})

		Context("to faulty endpoint", func() {
			BeforeEach(func() {
				endpoint = "/failure"
			})

			It("captures error code", func() {
				Expect(event.Attributes).To(HaveKeyWithValue("status", "500"))
			})

			It("captures route", func() {
				Expect(event.Attributes).To(HaveKeyWithValue("path", "/failure"))
			})
		})
	})
})
