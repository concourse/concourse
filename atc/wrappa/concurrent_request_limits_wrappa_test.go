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
	)

	BeforeEach(func() {
		fakeHandler = new(wrappafakes.FakeHandler)
		fakeLogger = new(wrappafakes.FakeLogger)
	})

	givenConcurrencyLimit := func(limit int) wrappa.Wrappa {
		return wrappa.NewConcurrencyLimitsWrappa(
			fakeLogger,
			wrappa.NewConcurrentRequestPolicy(
				[]wrappa.ConcurrentRequestLimitFlag{
					wrappa.ConcurrentRequestLimitFlag{
						Action: atc.ListAllJobs,
						Limit:  limit,
					},
				},
				[]string{atc.ListAllJobs},
			),
		)
	}

	It("logs when the concurrent request limit is hit", func() {
		wrappa := givenConcurrencyLimit(0)

		req, _ := http.NewRequest("GET", "localhost:8080", nil)
		wrappa.Wrap(map[string]http.Handler{
			atc.ListAllJobs: fakeHandler,
		})[atc.ListAllJobs].ServeHTTP(httptest.NewRecorder(), req)

		Expect(fakeLogger.InfoCallCount()).To(Equal(1), "no log emitted")
		logAction, _ := fakeLogger.InfoArgsForCall(0)
		Expect(logAction).To(Equal("concurrent-request-limit-reached"))
	})

	It("invokes the wrapped handler when the limit is not reached", func() {
		wrappa := givenConcurrencyLimit(1)

		req, _ := http.NewRequest("GET", "localhost:8080", nil)
		wrappa.Wrap(map[string]http.Handler{
			atc.ListAllJobs: fakeHandler,
		})[atc.ListAllJobs].ServeHTTP(httptest.NewRecorder(), req)

		Expect(fakeHandler.ServeHTTPCallCount()).To(Equal(1), "wrapped handler not invoked")
	})
})
