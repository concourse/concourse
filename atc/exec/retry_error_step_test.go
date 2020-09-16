package exec_test

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"

	. "github.com/concourse/concourse/atc/exec"
	"github.com/concourse/concourse/atc/exec/build"
	"github.com/concourse/concourse/atc/exec/execfakes"
	"github.com/concourse/concourse/atc/worker/transport"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("RetryErrorStep", func() {
	var (
		ctx    context.Context
		cancel func()

		fakeStep     *execfakes.FakeStep
		fakeDelegate *execfakes.FakeBuildStepDelegate

		repo  *build.Repository
		state *execfakes.FakeRunState

		step Step
	)

	BeforeEach(func() {
		ctx, cancel = context.WithCancel(context.Background())

		fakeStep = new(execfakes.FakeStep)
		fakeDelegate = new(execfakes.FakeBuildStepDelegate)

		repo = build.NewRepository()
		state = new(execfakes.FakeRunState)
		state.ArtifactRepositoryReturns(repo)

		step = RetryError(fakeStep, fakeDelegate)
	})

	AfterEach(func() {
		cancel()
	})

	Describe("Run", func() {
		var runErr error

		JustBeforeEach(func() {
			runErr = step.Run(ctx, state)
		})

		Context("when the inner step does not error", func() {
			BeforeEach(func() {
				fakeStep.RunReturns(nil)
			})

			It("returns nil", func() {
				Expect(runErr).To(BeNil())
			})

			It("does not log", func() {
				Expect(fakeDelegate.ErroredCallCount()).To(Equal(0))
			})
		})

		Context("when aborted", func() {
			BeforeEach(func() {
				fakeStep.RunReturns(context.Canceled)
			})

			It("propagates the error", func() {
				Expect(runErr).To(Equal(context.Canceled))
			})
		})

		Context("when worker disappeared", func() {
			cause := transport.WorkerMissingError{"some-worker"}
			BeforeEach(func() {
				fakeStep.RunReturns(cause)
			})

			It("should return retriable", func() {
				Expect(runErr).To(Equal(Retriable{cause}))
			})

			It("logs 'timeout exceeded'", func() {
				Expect(fakeDelegate.ErroredCallCount()).To(Equal(1))
				_, message := fakeDelegate.ErroredArgsForCall(0)
				Expect(message).To(Equal(fmt.Sprintf("%s, will retry ...", cause.Error())))
			})
		})

		Context("when url.Error error happened", func() {
			cause := &url.Error{Op: "error", URL: "err", Err: errors.New("error")}
			BeforeEach(func() {
				fakeStep.RunReturns(cause)
			})

			It("should return retriable", func() {
				Expect(runErr).To(Equal(Retriable{cause}))
			})
		})

		Context("when net.Error error happened", func() {
			cause := &net.OpError{Op: "read", Net: "test", Source: nil, Addr: nil, Err: errors.New("test")}
			BeforeEach(func() {
				fakeStep.RunReturns(cause)
			})

			It("should return retriable", func() {
				Expect(runErr).To(Equal(Retriable{cause}))
			})
		})

		Context("when the inner step returns any other error", func() {
			disaster := errors.New("disaster")

			BeforeEach(func() {
				fakeStep.RunReturns(disaster)
			})

			It("propagates the error", func() {
				Expect(runErr).To(Equal(disaster))
			})
		})
	})

	Describe("Succeeded", func() {
		Context("when the wrapped step has succeeded", func() {
			BeforeEach(func() {
				fakeStep.SucceededReturns(true)
			})

			It("returns true", func() {
				Expect(step.Succeeded()).Should(BeTrue())
			})
		})

		Context("when the wrapped step has failed", func() {
			BeforeEach(func() {
				fakeStep.SucceededReturns(false)
			})

			It("returns true", func() {
				Expect(step.Succeeded()).Should(BeFalse())
			})
		})
	})
})
