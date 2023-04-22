package exec_test

import (
	"context"
	"errors"
	"fmt"

	. "github.com/concourse/concourse/atc/exec"
	"github.com/concourse/concourse/atc/exec/build"
	"github.com/concourse/concourse/atc/exec/execfakes"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("LogErrorStep", func() {
	var (
		ctx    context.Context
		cancel func()

		fakeStep *execfakes.FakeStep

		fakeDelegate        *execfakes.FakeBuildStepDelegate
		fakeDelegateFactory *execfakes.FakeBuildStepDelegateFactory

		repo  *build.Repository
		state *execfakes.FakeRunState

		step Step
	)

	BeforeEach(func() {
		ctx, cancel = context.WithCancel(context.Background())

		fakeStep = new(execfakes.FakeStep)
		fakeDelegate = new(execfakes.FakeBuildStepDelegate)
		fakeDelegateFactory = new(execfakes.FakeBuildStepDelegateFactory)
		fakeDelegateFactory.BuildStepDelegateReturns(fakeDelegate)

		repo = build.NewRepository()
		state = new(execfakes.FakeRunState)
		state.ArtifactRepositoryReturns(repo)

		step = LogError(fakeStep, fakeDelegateFactory)
	})

	AfterEach(func() {
		cancel()
	})

	Describe("Run", func() {
		var runOk bool
		var runErr error

		JustBeforeEach(func() {
			runOk, runErr = step.Run(ctx, state)
		})

		Context("when the inner step does not error", func() {
			BeforeEach(func() {
				fakeStep.RunReturns(true, nil)
			})

			It("returns true", func() {
				Expect(runOk).Should(BeTrue())
			})

			It("returns nil", func() {
				Expect(runErr).To(BeNil())
			})

			It("does not log", func() {
				Expect(fakeDelegate.ErroredCallCount()).To(Equal(0))
			})
		})

		Context("when the inner step has failed", func() {
			BeforeEach(func() {
				fakeStep.RunReturns(false, nil)
			})

			It("returns false", func() {
				Expect(runOk).Should(BeFalse())
			})

			It("returns nil", func() {
				Expect(runErr).To(BeNil())
			})

			It("does not log", func() {
				Expect(fakeDelegate.ErroredCallCount()).To(Equal(0))
			})
		})

		Context("when aborted", func() {
			var canceled = fmt.Errorf("wrapped: %w", context.Canceled)

			BeforeEach(func() {
				fakeStep.RunReturns(false, canceled)
			})

			It("propagates the error", func() {
				Expect(runErr).To(Equal(canceled))
			})

			It("logs 'interrupted'", func() {
				Expect(fakeDelegate.ErroredCallCount()).To(Equal(1))
				_, message := fakeDelegate.ErroredArgsForCall(0)
				Expect(message).To(Equal("interrupted"))
			})
		})

		Context("when timed out", func() {
			var timedOut = fmt.Errorf("wrapped: %w", context.DeadlineExceeded)

			BeforeEach(func() {
				fakeStep.RunReturns(false, timedOut)
			})

			It("propagates the error", func() {
				Expect(runErr).To(Equal(timedOut))
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
				fakeStep.RunReturns(false, disaster)
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
})
