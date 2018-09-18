package exec_test

import (
	"context"
	"errors"

	. "github.com/concourse/atc/exec"
	"github.com/concourse/atc/worker"

	"github.com/concourse/atc/exec/execfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("LogErrorStep", func() {
	var (
		ctx    context.Context
		cancel func()

		fakeStep     *execfakes.FakeStep
		fakeDelegate *execfakes.FakeBuildStepDelegate

		repo  *worker.ArtifactRepository
		state *execfakes.FakeRunState

		step Step
	)

	BeforeEach(func() {
		ctx, cancel = context.WithCancel(context.Background())

		fakeStep = new(execfakes.FakeStep)
		fakeDelegate = new(execfakes.FakeBuildStepDelegate)

		repo = worker.NewArtifactRepository()
		state = new(execfakes.FakeRunState)
		state.ArtifactsReturns(repo)

		step = LogError(fakeStep, fakeDelegate)
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

			It("logs 'interrupted'", func() {
				Expect(fakeDelegate.ErroredCallCount()).To(Equal(1))
				_, message := fakeDelegate.ErroredArgsForCall(0)
				Expect(message).To(Equal("interrupted"))
			})
		})

		Context("when timed out", func() {
			BeforeEach(func() {
				fakeStep.RunReturns(context.DeadlineExceeded)
			})

			It("propagates the error", func() {
				Expect(runErr).To(Equal(context.DeadlineExceeded))
			})

			It("logs 'timeout exceeded'", func() {
				Expect(fakeDelegate.ErroredCallCount()).To(Equal(1))
				_, message := fakeDelegate.ErroredArgsForCall(0)
				Expect(message).To(Equal("timeout exceeded"))
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

			It("logs the error", func() {
				Expect(fakeDelegate.ErroredCallCount()).To(Equal(1))
				_, message := fakeDelegate.ErroredArgsForCall(0)
				Expect(message).To(Equal("disaster"))
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
