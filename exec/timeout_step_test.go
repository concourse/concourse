package exec_test

import (
	"context"
	"errors"
	"time"

	. "github.com/concourse/atc/exec"
	"github.com/concourse/atc/worker"

	"github.com/concourse/atc/exec/execfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Timeout Step", func() {
	var (
		ctx    context.Context
		cancel func()

		fakeStep *execfakes.FakeStep

		repo  *worker.ArtifactRepository
		state *execfakes.FakeRunState

		step Step

		timeoutDuration string

		stepErr error
	)

	BeforeEach(func() {
		ctx, cancel = context.WithCancel(context.Background())

		fakeStep = new(execfakes.FakeStep)

		repo = worker.NewArtifactRepository()
		state = new(execfakes.FakeRunState)
		state.ArtifactsReturns(repo)

		timeoutDuration = "1h"
	})

	JustBeforeEach(func() {
		step = Timeout(fakeStep, timeoutDuration)
		stepErr = step.Run(ctx, state)
	})

	Context("when the duration is valid", func() {
		It("runs the step with a deadline", func() {
			runCtx, _ := fakeStep.RunArgsForCall(0)
			deadline, ok := runCtx.Deadline()
			Expect(ok).To(BeTrue())
			Expect(deadline).To(BeTemporally("~", time.Now().Add(time.Hour), 10*time.Second))
		})

		Context("when the step returns an error", func() {
			var someError error

			BeforeEach(func() {
				someError = errors.New("some error")
				fakeStep.SucceededReturns(false)
				fakeStep.RunReturns(someError)
			})

			It("returns the error", func() {
				Expect(stepErr).NotTo(BeNil())
				Expect(stepErr).To(Equal(someError))
			})
		})

		Context("when the step exceeds the timeout", func() {
			BeforeEach(func() {
				fakeStep.SucceededReturns(true)
				fakeStep.RunReturns(context.DeadlineExceeded)
			})

			It("returns no error", func() {
				Expect(stepErr).ToNot(HaveOccurred())
			})

			It("is not successful", func() {
				Expect(step.Succeeded()).To(BeFalse())
			})
		})

		Describe("canceling", func() {
			BeforeEach(func() {
				cancel()
			})

			It("forwards the context down", func() {
				runCtx, _ := fakeStep.RunArgsForCall(0)
				Expect(runCtx.Err()).To(Equal(context.Canceled))
			})

			It("is not successful", func() {
				Expect(step.Succeeded()).To(BeFalse())
			})
		})

		Context("when the step is successful", func() {
			BeforeEach(func() {
				fakeStep.SucceededReturns(true)
			})

			It("is successful", func() {
				Expect(step.Succeeded()).To(BeTrue())
			})
		})

		Context("when the step fails", func() {
			BeforeEach(func() {
				fakeStep.SucceededReturns(false)
			})

			It("is not successful", func() {
				Expect(step.Succeeded()).To(BeFalse())
			})
		})
	})

	Context("when the duration is invalid", func() {
		BeforeEach(func() {
			timeoutDuration = "nope"
		})

		It("errors immediately without running the step", func() {
			Expect(stepErr).To(HaveOccurred())
			Expect(fakeStep.RunCallCount()).To(BeZero())
		})
	})
})
