package metric_test

import (
	"net/http"
	"net/http/httptest"

	"code.cloudfoundry.org/lager"
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

var dummyLogger = lager.NewLogger("dont care")

var _ = Describe("MetricsHandler", func() {
	var (
		ts      *httptest.Server
		emitter *metricfakes.FakeEmitter
	)

	BeforeEach(func() {
		emitterFactory := &metricfakes.FakeEmitterFactory{}
		emitter = &metricfakes.FakeEmitter{}

		metric.RegisterEmitter(emitterFactory)
		emitterFactory.IsConfiguredReturns(true)
		emitterFactory.NewEmitterReturns(emitter, nil)

		metric.Initialize(dummyLogger, "test", map[string]string{}, 1000)

		ts = httptest.NewServer(
			WrapHandler(dummyLogger, "ApiEndpoint", http.HandlerFunc(noopHandler)))
	})

	AfterEach(func() {
		ts.Close()
		metric.Deinitialize(dummyLogger)
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
