package wrappa_test

import (
	"net/http"
	"net/http/httptest"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/metric"
	"github.com/concourse/concourse/atc/wrappa"
	"github.com/concourse/concourse/atc/wrappa/wrappafakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
)

var _ = Describe("Concurrent Request Limits Wrappa", func() {
	var (
		fakeHandler *wrappafakes.FakeHandler
		fakePolicy  *wrappafakes.FakeConcurrentRequestPolicy
		fakePool    *wrappafakes.FakePool
		testLogger  *lagertest.TestLogger
		handler     http.Handler
		request     *http.Request
	)

	BeforeEach(func() {
		fakeHandler = new(wrappafakes.FakeHandler)
		testLogger = lagertest.NewTestLogger("test")
		request, _ = http.NewRequest("GET", "localhost:8080", nil)
	})

	AfterEach(func() {
		metric.Metrics.ConcurrentRequests = map[string]*metric.Gauge{}
	})

	givenConcurrentRequestLimit := func(limit int) {
		fakePolicy = new(wrappafakes.FakeConcurrentRequestPolicy)
		fakePool = new(wrappafakes.FakePool)
		fakePolicy.HandlerPoolReturns(fakePool, true)
		fakePool.SizeReturns(limit)

		handler = wrappa.NewConcurrentRequestLimitsWrappa(testLogger, fakePolicy).
			Wrap(map[string]http.Handler{
				atc.ListAllJobs: fakeHandler,
			})[atc.ListAllJobs]
	}

	Context("when the limit is reached", func() {
		BeforeEach(func() {
			givenConcurrentRequestLimit(1)
			fakePool.TryAcquireReturns(false)
		})

		It("responds with a 503", func() {
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)

			Expect(recorder.Code).To(Equal(http.StatusServiceUnavailable))
		})

		It("logs an INFO message", func() {
			handler.ServeHTTP(httptest.NewRecorder(), request)

			Expect(testLogger.Logs()).To(ConsistOf(
				MatchFields(IgnoreExtras, Fields{
					"Message":  Equal("test.concurrent-request-limit-reached"),
					"LogLevel": Equal(lager.INFO),
				}),
			))
		})

		It("increments the 'limitHit' counter", func() {
			handler.ServeHTTP(httptest.NewRecorder(), request)
			handler.ServeHTTP(httptest.NewRecorder(), request)

			Expect(metric.Metrics.ConcurrentRequestsLimitHit[atc.ListAllJobs].Delta()).To(Equal(float64(2)))
		})
	})

	Context("when the limit is not reached", func() {
		BeforeEach(func() {
			givenConcurrentRequestLimit(1)
			fakePool.TryAcquireReturns(true)
		})

		It("invokes the wrapped handler", func() {
			handler.ServeHTTP(httptest.NewRecorder(), request)

			Expect(fakeHandler.ServeHTTPCallCount()).To(Equal(1), "wrapped handler not invoked")
		})

		It("releases the pool", func() {
			handler.ServeHTTP(httptest.NewRecorder(), request)

			Expect(fakePool.ReleaseCallCount()).To(Equal(1))
		})

		It("records the number of requests in-flight", func() {
			handler.ServeHTTP(httptest.NewRecorder(), request)
			handler.ServeHTTP(httptest.NewRecorder(), request)

			Expect(metric.Metrics.ConcurrentRequests[atc.ListAllJobs].Max()).To(Equal(float64(1)))
		})
	})

	Context("when the endpoint is disabled", func() {
		BeforeEach(func() {
			givenConcurrentRequestLimit(0)
		})

		It("responds with a 501", func() {
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)

			Expect(recorder.Code).To(Equal(http.StatusNotImplemented))
		})

		It("logs a DEBUG message", func() {
			handler.ServeHTTP(httptest.NewRecorder(), request)

			Expect(testLogger.Logs()).To(ConsistOf(
				MatchFields(IgnoreExtras, Fields{
					"Message":  Equal("test.endpoint-disabled"),
					"LogLevel": Equal(lager.DEBUG),
				}),
			))
		})

		It("increments the 'limitHit' counter", func() {
			handler.ServeHTTP(httptest.NewRecorder(), request)
			handler.ServeHTTP(httptest.NewRecorder(), request)

			Expect(metric.Metrics.ConcurrentRequestsLimitHit[atc.ListAllJobs].Delta()).To(Equal(float64(2)))
		})
	})
})
