package transport_test

import (
	"errors"

	"github.com/concourse/atc/worker/transport"
	"github.com/concourse/retryhttp"
	"github.com/concourse/retryhttp/retryhttpfakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("StalledWorkerRetryer", func() {
	var (
		retryer         retryhttp.Retryer
		delegateRetryer *retryhttpfakes.FakeRetryer
	)

	BeforeEach(func() {
		delegateRetryer = &retryhttpfakes.FakeRetryer{}

		retryer = &transport.StalledWorkerRetryer{
			DelegateRetryer: delegateRetryer,
		}
	})

	Describe("IsRetryable", func() {
		It("returns true when error is ErrWorkerStalled", func() {
			err := transport.ErrWorkerStalled{WorkerName: "foo"}
			Expect(retryer.IsRetryable(err)).To(BeTrue())
		})

		It("delegates to DelegateRetryer if errors is not ErrWorkerStalled", func() {
			err := errors.New("some-other-error")
			delegateRetryer.IsRetryableReturns(true)
			Expect(retryer.IsRetryable(err)).To(BeTrue())
			Expect(delegateRetryer.IsRetryableCallCount()).To(Equal(1))
		})
	})
})
