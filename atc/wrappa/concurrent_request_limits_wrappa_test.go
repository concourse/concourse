package wrappa_test

import (
	"net/http"
	"net/http/httptest"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/wrappa"
	"github.com/concourse/concourse/atc/wrappa/wrappafakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
)

var _ = Describe("Concurrent Request Limits Wrappa", func() {
	var (
		fakeHandler *wrappafakes.FakeHandler
		testLogger  *lagertest.TestLogger
		handler     http.Handler
		request     *http.Request
		policy      wrappa.ConcurrentRequestPolicy
	)

	BeforeEach(func() {
		fakeHandler = new(wrappafakes.FakeHandler)
		testLogger = lagertest.NewTestLogger("test")
		request, _ = http.NewRequest("GET", "localhost:8080", nil)
	})

	givenConcurrentRequestLimit := func(limit int) {
		policy = wrappa.NewConcurrentRequestPolicy(
			map[wrappa.LimitedRoute]int{
				wrappa.LimitedRoute(atc.ListAllJobs): limit,
			},
		)
		handler = wrappa.NewConcurrentRequestLimitsWrappa(testLogger, policy).
			Wrap(map[string]http.Handler{
				atc.ListAllJobs: fakeHandler,
			})[atc.ListAllJobs]
	}

	It("logs when the concurrent request limit is hit", func() {
		givenConcurrentRequestLimit(0)

		handler.ServeHTTP(httptest.NewRecorder(), request)

		Expect(testLogger.Logs()).To(ConsistOf(
			MatchFields(IgnoreExtras, Fields{
				"Message":  Equal("test.concurrent-request-limit-reached"),
				"LogLevel": Equal(lager.INFO),
			}),
		))
	})

	It("invokes the wrapped handler when the limit is not reached", func() {
		givenConcurrentRequestLimit(1)

		handler.ServeHTTP(httptest.NewRecorder(), request)

		Expect(fakeHandler.ServeHTTPCallCount()).To(Equal(1), "wrapped handler not invoked")
	})

	It("permits serial requests", func() {
		givenConcurrentRequestLimit(1)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(httptest.NewRecorder(), request)
		handler.ServeHTTP(rec, request)

		Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))
	})
})
