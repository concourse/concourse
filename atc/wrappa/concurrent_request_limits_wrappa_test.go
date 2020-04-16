package wrappa_test

import (
	"net/http"
	"net/http/httptest"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/wrappa"
	"github.com/concourse/concourse/atc/wrappa/wrappafakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Concurrent Request Limits Wrappa", func() {
	var (
		fakeHandler *wrappafakes.FakeHandler
		fakeLogger  *wrappafakes.FakeLogger
		handler     http.Handler
		request     *http.Request
		policy      wrappa.ConcurrentRequestPolicy
	)

	BeforeEach(func() {
		fakeHandler = new(wrappafakes.FakeHandler)
		fakeLogger = new(wrappafakes.FakeLogger)
		request, _ = http.NewRequest("GET", "localhost:8080", nil)
	})

	givenConcurrentRequestLimit := func(limit int) {
		policy = wrappa.NewConcurrentRequestPolicy(
			map[wrappa.LimitedRoute]int{
				wrappa.LimitedRoute(atc.ListAllJobs): limit,
			},
		)
		handler = wrappa.NewConcurrentRequestLimitsWrappa(fakeLogger, policy).
			Wrap(map[string]http.Handler{
				atc.ListAllJobs: fakeHandler,
			})[atc.ListAllJobs]
	}

	It("logs when the concurrent request limit is hit", func() {
		givenConcurrentRequestLimit(0)

		handler.ServeHTTP(httptest.NewRecorder(), request)

		Expect(fakeLogger.InfoCallCount()).To(Equal(1), "no log emitted")
		logAction, _ := fakeLogger.InfoArgsForCall(0)
		Expect(logAction).To(Equal("concurrent-request-limit-reached"))
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

	It("logs error when the pool fails to release", func() {
		givenConcurrentRequestLimit(1)
		fakeHandler.ServeHTTPStub = func(http.ResponseWriter, *http.Request) {
			pool, _ := policy.HandlerPool(atc.ListAllJobs)
			pool.Release()
		}

		handler.ServeHTTP(httptest.NewRecorder(), request)

		Expect(fakeLogger.ErrorCallCount()).To(Equal(1))
		logAction, _, _ := fakeLogger.ErrorArgsForCall(0)
		Expect(logAction).To(Equal("failed-to-release-handler-pool"))
	})

	It("does not log error when releasing succeeds", func() {
		givenConcurrentRequestLimit(1)

		handler.ServeHTTP(httptest.NewRecorder(), request)

		Expect(fakeLogger.ErrorCallCount()).To(Equal(0))
	})
})
