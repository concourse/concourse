package transport_test

import (
	"errors"

	"github.com/concourse/atc/worker/transport"
	"github.com/concourse/retryhttp"
	"github.com/concourse/retryhttp/retryhttpfakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("UnreachableWorkerRetryer", func() {
	var (
		retryer         retryhttp.Retryer
		delegateRetryer *retryhttpfakes.FakeRetryer
	)

	BeforeEach(func() {
		delegateRetryer = &retryhttpfakes.FakeRetryer{}

		retryer = &transport.UnreachableWorkerRetryer{
			DelegateRetryer: delegateRetryer,
		}
	})

	Describe("IsRetryable", func() {
		It("returns true when error is WorkerUnreachableError", func() {
			err := transport.WorkerUnreachableError{
				WorkerName:  "foo",
				WorkerState: "stalled",
			}

			Expect(retryer.IsRetryable(err)).To(BeTrue())
		})

		It("delegates to DelegateRetryer if errors is not WorkerUnreachableError", func() {
			err := errors.New("some-other-error")
			delegateRetryer.IsRetryableReturns(true)
			Expect(retryer.IsRetryable(err)).To(BeTrue())
			Expect(delegateRetryer.IsRetryableCallCount()).To(Equal(1))
		})
	})
})
