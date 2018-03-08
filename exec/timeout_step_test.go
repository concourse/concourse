package exec_test

import (
	"context"
	"errors"
	"time"

	. "github.com/concourse/atc/exec"

	"github.com/concourse/atc/exec/execfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Timeout Step", func() {
	var (
		ctx    context.Context
		cancel func()

		fakeStepFactoryStep *execfakes.FakeStepFactory

		runStep *execfakes.FakeStep

		timeout StepFactory
		step    Step

		timeoutDuration string

		stepErr error
	)

	BeforeEach(func() {
		ctx, cancel = context.WithCancel(context.Background())

		fakeStepFactoryStep = new(execfakes.FakeStepFactory)
		runStep = new(execfakes.FakeStep)
		fakeStepFactoryStep.UsingReturns(runStep)

		timeoutDuration = "1h"
	})

	JustBeforeEach(func() {
		timeout = Timeout(fakeStepFactoryStep, timeoutDuration)
		step = timeout.Using(nil)
		stepErr = step.Run(ctx)
	})

	Context("when the duration is valid", func() {
		It("runs the step with a deadline", func() {
			deadline, ok := runStep.RunArgsForCall(0).Deadline()
			Expect(ok).To(BeTrue())
			Expect(deadline).To(BeTemporally("~", time.Now().Add(time.Hour), 10*time.Second))
		})

		Context("when the step returns an error", func() {
			var someError error

			BeforeEach(func() {
				someError = errors.New("some error")
				runStep.SucceededReturns(false)
				runStep.RunReturns(someError)
			})

			It("returns the error", func() {
				Expect(stepErr).NotTo(BeNil())
				Expect(stepErr).To(Equal(someError))
			})
		})

		Context("when the step exceeds the timeout", func() {
			BeforeEach(func() {
				runStep.SucceededReturns(true)
				runStep.RunReturns(context.DeadlineExceeded)
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
				Expect(runStep.RunArgsForCall(0).Err()).To(Equal(context.Canceled))
			})

			It("is not successful", func() {
				Expect(step.Succeeded()).To(BeFalse())
			})
		})

		Context("when the step is successful", func() {
			BeforeEach(func() {
				runStep.SucceededReturns(true)
			})

			It("is successful", func() {
				Expect(step.Succeeded()).To(BeTrue())
			})
		})

		Context("when the step fails", func() {
			BeforeEach(func() {
				runStep.SucceededReturns(false)
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
			Expect(runStep.RunCallCount()).To(BeZero())
		})
	})
})
