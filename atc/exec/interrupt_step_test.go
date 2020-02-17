package exec_test

import (
	"context"
	"errors"

	. "github.com/concourse/concourse/atc/exec"
	"github.com/concourse/concourse/atc/exec/build"
	"github.com/concourse/concourse/atc/exec/execfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Interrupt Step", func() {
	var (
		ctx context.Context

		fakeStep *execfakes.FakeStep

		repo  *build.Repository
		state *execfakes.FakeRunState

		step Step

		interruptDuration string

		stepErr error
	)

	BeforeEach(func() {
		ctx = context.Background()

		fakeStep = new(execfakes.FakeStep)

		repo = build.NewRepository()
		state = new(execfakes.FakeRunState)
		state.ArtifactRepositoryReturns(repo)

		interruptDuration = "1h"
	})

	JustBeforeEach(func() {
		step = Interrupt(fakeStep, interruptDuration)
		stepErr = step.Run(ctx, state)
	})

	Context("when the duration is valid", func() {
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
			interruptDuration = "nope"
		})

		It("errors immediately without running the step", func() {
			Expect(stepErr).To(HaveOccurred())
			Expect(fakeStep.RunCallCount()).To(BeZero())
		})
	})
})
